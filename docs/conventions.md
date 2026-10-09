# Conventions

How a SpecArch specification is laid out on disk, how the YAML is organised,
what the Markdown sections are, and which diagrams are generated.

## The tree

A project keeps its specification in a `spec/` folder at the repository root
(or wherever its own rules say). The specification is that folder: its root
file `specarch.yaml`, and one folder per life-cycle stage it keeps.

    spec/
      specarch.yaml                  the root: specarch, info, stages, sources
      specarch.md                    the hand-written explanation, with generated regions
      requirements/                  stakeholders, needs, requirements, glossary, assumptions, constraints
      design/                        enums, entities, permissions, roles, paths, commands, channels, pages, algorithms, decisions
      implementation/<stack>/        <name>.<stack>.specarch-implementation.yaml, and its .md when it needs one
      tests/<name>/                  test.yaml, and the scenario's own input and expected-output files
      deployment/                    environments, configuration, release, rollback, migrations
      commissioning/                 checks, signoff
      operation/                     monitors
      <stage>/questions.yaml         the open questions about that stage, in any stage folder

The rules, each of which the validator checks under the rule `layout`:

- The root file lists the stages it keeps under `stages`, in life-cycle
  order. A listed stage has a folder of that name; a folder that is not a
  listed stage is refused; a listed stage without a folder is refused.
- A stage that is not listed has no folder. Its sections, if it has any,
  are written in the root file itself; an implementation file then sits
  beside the root file. So a tiny specification is one file, and a project
  adds a folder when a stage grows.
- A file under a stage folder is a mapping whose keys are sections of that
  stage, each holding named objects: `entities: { Loan: ... }`. A file may
  hold one object or a few. A sub-folder named after a section holds only
  that section. The convention, used in `spec/` and the example, is one
  sub-folder per section and one object per file, named after the object
  in kebab-case (`design/entities/loan.yaml`, `design/decisions/ADR-001.yaml`),
  with the small sections (`permissions`, `roles`) in one file each. The
  single-object sections (`release`, `rollback`, `signoff`) are one file
  each, `deployment/release.yaml`.
- Each test is a folder `tests/<name>/` holding `test.yaml`, the test
  itself, beside any input and expected-output files the scenario needs;
  the validator reads nothing in a test folder but `test.yaml`. A file
  directly under `tests/` is refused.
- An implementation file lives under `implementation/<stack>/`, and the
  folder's name is the stack in the file's name.
- Files end in `.yaml`; `.md` and other files in a stage folder are left
  alone. Folders starting with a dot are skipped.

The validator reads the tree as one document, in the shape the schema
describes: the root file's keys, then every section merged from its files.
A name is defined once within its section across the whole tree; a second
definition is refused with both files named. Because the document is one,
every reference is the plain key it would be in one file: `target: Loan`,
`$ref: "#/entities/Loan"`, `satisfies: [LIB-3]`, `source: iso-29148`. No
reference ever names a file, so objects move between files without
breaking one. A diagnostic names the file and line of the node and the
YAML path in the merged document, which is also the path an implementation
file's pointers use.

The first line of the root file is the editor hint:

    # yaml-language-server: $schema=https://raw.githubusercontent.com/SpecArch/specarch/v0.5.0/schema/specarch-design-0.1.schema.json

For an implementation file:

    # yaml-language-server: $schema=https://raw.githubusercontent.com/SpecArch/specarch/v0.5.0/schema/specarch-implementation-0.1.schema.json

The design schema describes the merged document, so a file under a stage
folder, which holds only some of its sections, has a schema of its own:

| File | Its schema |
|---|---|
| a file under `requirements/`, `design/`, `deployment/`, `commissioning/` or `operation/` | `specarch-fragment-<stage>-0.1.schema.json`, that stage's sections and `questions` |
| `tests/<name>/test.yaml` | `specarch-fragment-test-0.1.schema.json`, one test |
| a file directly under `tests/` or `implementation/` | `specarch-fragment-questions-0.1.schema.json`, only `questions` |

A fragment's hint names the same commit or tag as the root file's, here
for a file under `design/entities/`:

    # yaml-language-server: $schema=https://raw.githubusercontent.com/SpecArch/specarch/<commit or tag>/schema/specarch-fragment-design-0.1.schema.json

Each is written from the design schema by `tools/fragment-schemas`, which
copies the stage's sections and the definitions they reach, so a keyword
is defined once; `go test ./...` fails when the design schema changes and
they were not written again (`go run ./tools/fragment-schemas`). One file
alone cannot show what spans files: a reference to an object in another
file, a name defined twice, a sub-folder holding a second section, and a
required key an open must question blocks, which the editor reports as
missing and `specarch validate` excuses while the question is open. The
editor checks each file as it is written; `specarch validate` checks the
whole tree.

The specifications in this repository name the schemas by a path relative
to the file (`../../../schema/specarch-fragment-design-0.1.schema.json`),
because they are written against the schemas beside them rather than a
released tag; `go run ./tools/fragment-schemas` writes those lines too.

Pin the schema version a project is written against. A project moves to a
new meta-model version on purpose, in its own change.

## Design and implementation

A specification has two sides, and every fact belongs in exactly one of
them.

The **SpecArch Definition** (SDF) is the design side: the root file and
every stage folder but `implementation/`. It is neutral about language,
compiler and framework: stakeholders, needs and requirements; entities,
enums, operations, commands, events, pages, permissions and roles,
algorithms with formula, worked examples and pseudocode; tests; environments,
settings, release, rollback and migration steps; commissioning checks and
sign-off; monitors of the live system; and the decisions that hold whatever
the stack.

A **SpecArch Implementation File** (SIF, `<name>.<stack>.specarch-implementation.yaml`,
root key `specarchImplementation`) is one stack's implementation of one
specification. It names the root file and the `info.version` it was written
against under `implements`, and holds everything that depends on the stack
and nothing else: the language and toolchain with versions (`stack`), every
library with its version and licence (`libraries`), the package layout and
the design objects each package implements (`layout`), how each design
object maps onto the stack (`mappings`, each of which may carry `why`,
`cites` and `origin` like any element, and `ownedBy`, below), the framework per interface kind
(`bindings`), the document and code targets with their folders and settings
(`targets`), build, test and CI commands (`tasks`), the test suites with
their levels (`testing`), the shipped idioms it excludes or overrides, each
with the reason (`idioms`), real servers, hosts, ports and the values of
non-secret settings per environment (`deployments`), and implementation
decisions (`decisions`). The idioms SpecArch ships, under `idioms/`, apply
to every implementation file by default; an override or a project's own
idiom is a file in the `idioms/` folder beside the implementation file,
checked against `schema/specarch-idiom-0.1.schema.json`. `docs/idioms.md`
is the design, and `specarch idioms` lists what each file uses. Its schema,
`schema/specarch-implementation-0.1.schema.json`, has no keyword for an
entity, an operation or any other design object, so an implementation file
cannot add or change design. One specification can have several
implementation files, one per stack; the `<stack>` part of the name tells
them apart (`library-lending.go.specarch-implementation.yaml`).

Pointers from an implementation file into its specification are JSON
pointers written from the merged document's root: `#/entities/Loan`,
`#/commands/validate`, `#/paths/~1loans/post` (a `/` inside a key is
written `~1`). The validator checks that `implements` names the root file
at the same `info.version` and that every pointer resolves, so a design
change is noticed by every implementation of it.

Targets read the specification for the design and the implementation file
for the target choices: which folder a target owns, the settings of the
code generator that follows it, the type a decimal maps to.

An element another stakeholder owns, such as a table another team keeps
in the same database or a route another service answers behind the same
gateway, is marked on its mapping with `ownedBy`, naming a stakeholder of
the specification:

```yaml
mappings:
  "#/entities/Book":
    target: table books
    ownedBy: catalogue-team
```

The specification still describes the element: validate, diff, gaps, the
documents and the generation gate read it as they read any other, and so
do the generated tests, which check the running system. No generator that
builds code or data writes it, nor anything under its pointer; an element that
refers to it, such as a foreign key or a schema reference, still refers
to it. The mark is per mapping because the parts another team owns are
rarely a whole file, and it sits in the implementation file because who
builds what is a choice of one implementation, not part of the design
(ADR-046). An `ownedBy` that names no stakeholder is the error
`stakeholder`.

### The interface boundary

The rule of thumb: if a client of the interface needs to know it, it is
design and goes in the specification. If only the people building or
running one particular implementation need it, it goes in the
implementation file.

| Specification (design) | Implementation file (one stack) |
|---|---|
| the kind of interface: HTTP (`paths`), messaging (`channels`), command line (`commands`) | the framework binding: router, middleware, how handlers implement the generated interface; the CLI library |
| operations with operationId, method and path; parameters, request bodies, responses with schemas, status codes and error shapes | code-generator configuration: package names, strict-server mode, `x-oapi-codegen-*` extensions, protoc plugins |
| channels and messages | the messaging client and its settings |
| commands, arguments, options and exit codes | |
| the permission each operation, command and page needs; roles | |
| environments and what each is for; the settings and which are secret; release, rollback and migration steps | servers, hosts, ports and the values of the non-secret settings per deployment; the exact commands |
| the commissioning checks and the sign-off | |
| the monitors: what is measured, in which environment, and the objective | the tool that measures, how, and where an alert goes, per monitor in each deployment |

A standalone `openapi.yaml` or `asyncapi.yaml` is made from the
specification. It is never a third source kept by hand.

Both schemas enforce the boundary. The specification schema refuses extension
keys that name one implementation: `x-oapi-codegen-*`, `x-go-*` and the
other language prefixes, `x-server`, `x-host`, `x-port`, `x-deploy`,
`x-router`, `x-framework` and the rest of the `stackSpecificKey` pattern in
the schema. The implementation schema is closed and has no design keyword.
The validator reports each case with its own rule, `stack_key` or
`design_key`. Other interface kinds (gRPC, file exchange, a serial protocol)
have no keyword yet; they are items in `docs/roadmap.md`.

### No change log inside a file

A specification or implementation file says what is true now. Text such as
"previously", "changed from", "updated on" or "new in" does not belong in a
description; the history of a project lives in its own history files and in
git. Version fields stay, because they describe the current file. The
validator warns on the most common change-log phrases (`change_log`).

## Why, citations and origin

Every element at every stage may carry three optional fields, the same
everywhere:

- `why`: the rationale in plain words, why the element is the way it is
  and what was concluded. A document renders it as an Insight next to the
  element.
- `cites`: a list of citations, each naming a `source` declared in the root
  file, the `clause` where relevant, and what the source `says` that
  applies here. A document renders each as a Note under the Insight.
- `origin`: how the element is known. `stated`: a source says it, and
  `cites` names the source and where in it (`origin_citation` otherwise).
  `inferred`: it was concluded from evidence, and `why` says from what
  (`origin_reason`). `decided`: a stakeholder settled it, and `decidedIn`
  names the accepted decision (`origin_decision`). An element without
  `origin` was written spec-first. A specification built from sources says
  `tracksOrigin: true` in `info`; the validator then warns for every element
  of a section without an origin (`origin_missing`). A document renders the
  origin as one line, **Origin:**, before the Insight. `docs/refinement.md`
  is the design.

An Insight is one paragraph that starts with **Insight:**, a Note one that
starts with **Note:** and reads "From <title>, <edition>, clause <clause>:
<what it says>", with the source's URL after it when there is one. They
follow the description of an element that has its own heading. An element
shown as a row of a table gets them after the table, labelled with the
row's name: **Insight on LIB-5:**. A document whose Notes cite sources ends
with a Sources table of exactly those. The traceability matrix names
elements by ID only, so it carries none.

Sources are declared once under `sources` in the root file, keyed by
kebab-case name, with their `kind` (standard, regulation, document,
interview, system, code, or requirement-set, change-set or defect-set for
an external tracker, each with its `prefix`), `title`, `edition`, `author`,
`date` and `url`, and `givenOutside: true` when the system's owners gave
it to parties outside, such as a published interface or a contract, which
makes `specarch merge` ask a `must` question about an element only it or
only the code has. A citation of a source that is not
declared is refused (`source`). A `system` is a running system that was
observed; `code` is a system's source code, read at the commit its
`edition` names, and a citation of it names a file and line, or a package
and function, as its clause. A source may list its `clauses`, each with a
`title`: the numbered sections of a document, or the files and folders of
code. `specarch gaps` then shows which elements each clause produced and
which clauses produced nothing; a citation falls under the longest listed
clause it equals or starts with, followed by a dot, a colon, a slash or a
space. `docs/from-sources.md` is the procedure that uses them. Decisions keep their context, decision and
consequences: the context is what was true and at stake, `why` the
reasoning that led from it to the decision. A decision that answers open
questions names them under `answers` and the stakeholder who decided under
`decidedBy`; an answer given in words is declared as a source of kind
`interview` with its date.

## Open questions

What the sources do not say is not invented and not left blank: it is an
open question, in the section `questions`, the one section every stage
folder may hold. A question is written in the folder of the stage it is
about: beside the elements it blocks, as `questions.yaml` or under a
`questions/` sub-folder, and as a file directly under `tests/` or
`implementation/`, whose other entries are folders. A stage with no folder
has its questions in the root file.

    questions:
      Q-12:
        question: What is the loan period for a reference copy?
        kind: decision
        priority: must
        blocks: ["#/entities/Loan/properties/dueOn"]
        decidedBy: head-librarian
        options: ["14 days, as for an ordinary copy", "Reference copies are not lent"]
        why: The policy sheet gives periods for ordinary copies only.

Each question says what is asked, its `kind` (`decision`, or `material` to
be provided), its `priority` (`must`: what it blocks is not defined;
`should`: what it blocks is inferred and must be confirmed; `could`: the
answer would help but nothing waits), what it `blocks` (a stage, a section,
or a pointer to an element or to one key of it), who decides (`decidedBy`,
a stakeholder key), and the `options` when the answer is a choice. The ID
is an upper-case prefix of one or more letters, a dash and a number.

The question is the one licence for an element to be incomplete. A
required key missing at or under a pointer a `must` question blocks is
covered: the validator reports neither it nor the warnings about that
element, and `specarch gaps` lists the missing keys under the question. An
element known only by name, such as an entity or one of its fields, is
written as an empty mapping and blocked by pointer. Nothing else is covered: a wrong value beside the gap is still an
error, and a `should` question covers no missing key.

The validator checks that every `blocks` entry resolves (`question_block`),
that a question sits in the stage of what it blocks and blocks one stage
(`question_stage`), that `decidedBy` is a stakeholder (`stakeholder`), and
that no accepted decision `answers` a question still present
(`question_answered`). `validate` counts the open questions in its summary
line and lists none; `specarch gaps` and the document `questions` list them
by stage, with what each holds up and which outputs are ready, drafts or
waiting. A question is removed when it is answered; the answer is a
decision with `answers` and `decidedBy`, and the elements it settled carry
`origin: decided`.

## YAML layout

Top-level keys appear in this order, which is life-cycle order. A target
does not care; a reader does. In a tree, each section is in its stage's
folder and only the first four are in the root file.

1. `specarch`, `info`, `stages`, `sources`
2. `stakeholders`, `needs`, `requirements`, `glossary`, `assumptions`, `constraints`
3. `enums`
4. `entities`
5. `permissions`, `roles`, `session`
6. `paths`
7. `commands`
8. `channels`, `dependencies`, `jobs`, `workflows`, `errors`
9. `pages`, `menus`
10. `algorithms`
11. `tests`
12. `decisions`
13. `environments`, `configuration`, `release`, `rollback`, `migrations`
14. `checks`, `signoff`
15. `monitors`

Inside an entity: `description`, `type`, `properties`, `required`,
`primaryKey`, `relations`, `constraints`, `stateField`, `transitions`,
`satisfies`, `why`, `cites`. Fields appear in the order they would be shown
on a form: identity first, then the fields a person fills in, then the
fields the system sets.

Short objects are written on one line with flow style:
`{ type: string, format: uuid, readOnly: true }`. Anything with a
`description` longer than a few words goes block style. Prose longer than one
line uses a literal block (`|`).

Three values are always quoted, because YAML would otherwise read them as
something else: dates (`date: "2026-10-07"`, which YAML takes for a
timestamp), decimal amounts (`"0.50"`, which YAML takes for a float), and any
flow-style string containing a comma, a colon or a question mark. The schema
rejects the unquoted forms, so the mistake is caught, but it is better not to
make it. A `says` or `statement` with a comma in it is written in block
style or in double quotes.

### Names

| Thing | Case | Example |
|---|---|---|
| entity, enum, message | PascalCase | `Loan`, `LoanStatus`, `LoanCreated` |
| field, relation, algorithm, operationId, input, setting | camelCase | `dueOn`, `createLoan`, `dailyRate` |
| enum value | snake_case | `set_null`, `overdue` |
| constraint of an entity | snake_case, prefixed with the entity | `loan_due_after_loaned` |
| permission | dotted lower-case, `area.verb` | `loans.create` |
| role, page, stakeholder, source, environment, migration, check, test | kebab-case | `librarian`, `members-list`, `iso-29148`, `production` |
| channel | dotted lower-case | `loan.lifecycle` |
| command | kebab-case words separated by spaces | `validate`, `document techspec` |
| command argument, option | kebab-case | `paths`, `out` |
| decision | `ADR-` and three or more digits | `ADR-001` |
| requirement, need, assumption, constraint | an upper-case prefix, a dash and an ID | `LIB-5`, `NEED-1`, `ASSUME-1`, `CON-1` |
| glossary term | as written | `late fee` |

The schema enforces these patterns. Targets rely on them to derive names in
the output (table names, URL segments, constant names) without a mapping table.

A field keeps its camelCase name in the specification however the
interface writes it. An API whose JSON names its properties in snake_case
says so once, with `info.wireNames: snake_case`: every property of every
entity, view, body, parameter schema and message payload then goes on the
wire in snake case (`loanedOn` is `loaned_on`, `userID` and `userId` are
both `user_id`), and the validator refuses two properties of one object
that would meet there (`wire_name`). A parameter's name is written as it
is on the wire, and is not mapped (ADR-062).

In SQL, a table is the entity's name in snake case (`LoanStatus` is
`loan_status`), unless the implementation file's mapping names it
(`target: table loans`); a column is the field's name in snake case
(`dueOn` is `due_on`). A primary key is `pk_<table>`, the check of a
type-rendering row `ck_<table>_<column>`, a foreign key
`fk_<table>_<column>`, and a declared unique or check constraint keeps its
own name. Identifiers are written unquoted and in lower case.

### Types

Every value has a concrete type, and the type says how wide it is. A design
that only says "number" leaves each implementation to guess, and the guesses
differ: in JavaScript every number is a 64-bit float, so an int64 ID above
2^53 silently loses its last digits, and `1` and `"1"` are easy to mix up.
Stating the concrete type is what makes a correct implementation possible.

A field's `type` says how the value travels in JSON; its `format` says what
the value is. The types, one line each:

| Type | Written as | Holds |
|---|---|---|
| int32 | `type: integer, format: int32` | a signed 32-bit integer; safe as a JSON number everywhere |
| int | `type: integer, format: int64` | a signed 64-bit integer, as a JSON number; needs `minimum` and `maximum` within plus or minus 2^53 - 1 (9007199254740991) |
| int, carried as text | `type: string, format: int64` | a signed 64-bit integer that may exceed 2^53, carried as a JSON string of digits |
| uint | `type: integer, format: uint64` or `type: string, format: uint64` | an unsigned 64-bit integer, with the same rule for 2^53 |
| double | `type: number, format: double` | a 64-bit IEEE float: approximate, never for money |
| decimal | `type: string, format: decimal, precision: P, scale: S` | an exact decimal of P digits, S after the point, carried as a JSON string |
| string | `type: string` | text; a string of digits (a card number, a postcode) is a string, never a number |
| bool | `type: boolean` | true or false |
| bytes | `type: string, format: byte` | binary data, base64 in JSON |
| date | `type: string, format: date` | a calendar date, `"2026-10-07"` |
| timestamp | `type: string, format: date-time` | an instant, RFC 3339, `"2026-10-07T09:30:00Z"` |
| duration | `type: string, format: duration` | a length of time, ISO 8601, `"PT2H"` |
| time of day | `type: string, format: time` | a time of day without a date, RFC 3339 partial time, `"09:30:00"` |
| enum | `$ref: "#/enums/Name"` | one of the enum's values, carried as a string |

The schema refuses an `integer` without `int32`, `int64` or `uint64`, a
`number` without `double`, and a decimal without `precision` and `scale`.
The validator refuses an int64 or uint64 carried as a JSON number without
bounds inside 2^53 (`unsafe_integer`): either bound it, or carry it as a
string. `format` keeps its JSON Schema and OpenAPI meaning; `uint64` and
`decimal` are SpecArch's additions to the format list, because neither
standard names them.

### Where each keyword comes from

Every keyword in the meta-model is either borrowed from a standard, with its
standard meaning, or marked as SpecArch's own. Borrowed keywords are never
redefined.

| Keyword | Origin | Notes |
|---|---|---|
| `info`, `paths`, `parameters`, `requestBody`, `responses`, `content`, `operationId`, `summary`, `deprecated` | OpenAPI 3 | bounded subset: `in` is path, query or header; no callbacks, links or servers |
| `type`, `title`, `properties`, `required`, `enum`, `const`, `format`, `default`, `examples`, `minimum`, `maximum`, `exclusiveMinimum`, `exclusiveMaximum`, `multipleOf`, `minLength`, `maxLength`, `pattern`, `items`, `minItems`, `maxItems`, `uniqueItems`, `readOnly`, `writeOnly`, `$ref` | JSON Schema 2020-12 | `$ref` is restricted to `#/entities/X`, `#/enums/X`, `#/views/X` and `#/schemas/X`; nullability is written `type: [string, "null"]` as JSON Schema does |
| `channels`, `messages`, `payload` | AsyncAPI | one level: channel, messages, payload; no servers, bindings or operations objects |
| `x-*` | OpenAPI convention | allowed in every object, ignored by validation |
| `stages`, `sources`, `givenOutside` | SpecArch | the root file's list of stage folders and its registry of cited sources, each marked when it was given to parties outside |
| `stakeholders`, `needs`, `requirements`, `statement`, `priority`, `status`, `acceptance`, `verification` | SpecArch, after ISO/IEC/IEEE 29148 | the requirements stage: the standard's stakeholders (5.2.2), needs (6.3), requirements (6.4) and attributes (5.2.8); priority after MoSCoW; verification methods after MIL-STD-961E |
| `glossary`, `assumptions`, `constraints` (top level) | SpecArch, after ISO/IEC/IEEE 29148 | 9.2.3, 9.5.19 and 9.6.16 of the standard; arc42 sections 2 and 12 |
| `satisfies`, `verifies` | SpecArch, after SysML and 29148 6.5 | the satisfy and verify relations from a design element or a test to a requirement |
| `why`, `cites`, `source`, `clause`, `says` | SpecArch | the rationale and the citations every element may carry; 29148 (5.2.8) names rationale and source as attributes |
| `primaryKey`, `relations`, `constraints`, `stateField`, `transitions` | SpecArch | data-model concepts JSON Schema has no words for |
| `where` (of a unique constraint) | SpecArch, after PostgreSQL's partial index and SQL Server's filtered index | the records a unique constraint holds among; ISO SQL leaves indexes out, so no standard names it |
| `enabledBy` | SpecArch | the boolean setting that switches an operation, command or page on |
| `release` (of a requirement) | SpecArch | the release a requirement is meant for, by the version of its release record, as a change or a defect names the release it shipped in |
| `precision`, `scale` | SpecArch | decimal size; JSON Schema has no decimal type, so `format: decimal` on a string carries them |
| `valueDescriptions` | SpecArch | per-value meaning of an enum |
| `commands`, `arguments`, `options`, `reads`, `writes`, `standardOutput`, `standardError`, `exitCodes`, `repeatable` | SpecArch | command-line interfaces; no standard describes one |
| `permissions`, `roles`, `permission` | SpecArch | OpenAPI's `security` names a scheme, not a right; SpecArch needs the right |
| `separationOfDuties`, `cardinality` | ANSI INCITS 359 (RBAC), static separation of duty | a set and a number no holder may reach; stated over permissions, not roles, because the specification declares the duties and the roles that grant them |
| `session`, `idleTimeout`, `absoluteTimeout` | SpecArch, after the OWASP Session Management Cheat Sheet | how a signed-in session ends; the two timeouts are the cheat sheet's |
| `dependencies`, `timeout`, `calls` | SpecArch | external systems an operation calls, each with a time limit per call |
| `idempotencyKey` | SpecArch, after RFC 9110 9.2.2 and the IETF Idempotency-Key header draft | a header that tells a repeated request from a new one, on a method that is not idempotent by itself |
| `validity`, `from`, `until` | SpecArch | the fields that bound when a record is current |
| `guard`, `precondition`, `recordsChanged` | SpecArch | what is checked together with a data change |
| `sensitivity`, `atRest`, `lookup`, `audited`, `deletion` | SpecArch | how sensitive a field is and how it is stored, and what the system keeps about each record of an entity |
| `listOf`, `searchable`, `filterable`, `sortable`, `pageSize` | SpecArch | a list's whitelists and page size, which a client must know |
| `limits`, `maxRequestBytes`, `rate`, `requests`, `per`, `burst` | SpecArch | the request size and rate a client keeps to |
| `jobs`, `trigger`, `schedule`, `every`, `consumes`, `role`, `retries`, `limit`, `then` | SpecArch | work the system does on its own; a schedule is the five fields of cron, in UTC |
| `menus`, `title`, `page`, `items` | SpecArch | the navigation: a tree whose leaves open pages |
| `workflows`, `trigger`, `subject`, `steps`, `approvers`, `deadline`, `onDeadline`, `escalateTo` | BPMN 2.0, a sequential subset | a request that finishes after people approve it: an approval is a user task with its potential owners and a timer, an operation step a service task |
| `views`, `from`, `path`, `count`, `rows` | SpecArch, after the SQL view of ISO/IEC 9075 | a read model: an entity's row with fields read through its relations, counts and the rows of a one-to-many relation added, never written |
| `schemas` | OpenAPI 3, `components.schemas` | value objects: named object schemas with no key and no table, data passed around but not stored |
| `errors`, `status`, `title`, `condition`, `type`, `problem` | RFC 9457, Problem Details for HTTP APIs | the catalogue of problem types; `condition` is SpecArch's, the standard's other members are the document's own at run time |
| a duration (`timeout`, `idleTimeout`, `absoluteTimeout`) | ISO 8601, the form JSON Schema's `format: duration` names | days, hours, minutes and seconds only (`PT5S`, `P1DT12H`): weeks, months and years depend on the calendar, so a limit written in them would not mean the same every day |
| `emits`, `algorithm` | SpecArch | links from an operation to its events and its computation |
| `pages`, `kind`, `route`, `entity`, `source`, `submit`, `columns`, `fields`, `filters`, `actions` | SpecArch | UI page definitions; a page of kind `task` submits to an operation without loading a record |
| `onSubmitted`, `onSelect`, `then`, `navigate`, `with`, `message` | SpecArch, after the events and navigation flows of OMG IFML 1.0 | where an event of a page leads, and the status message it carries (WCAG 2.2, 4.1.3) |
| `flows`, `actor`, `steps`, `event`, `action` | SpecArch, after IFML's navigation flows | a task a person does across pages, step by step |
| `states`, `empty`, `filteredEmpty`, `failed`, `message`, `field` | SpecArch | what a page shows when it is empty or fails |
| `sections`, `title`, `fields` | SpecArch | a form's or a view's fields in titled groups, in reading order |
| `compactColumns` | SpecArch, after the compact size class of Apple's Human Interface Guidelines and Material Design 3 | the columns a list keeps on a compact screen |
| `childRows`, `relation`, `maximum`, `lockLoadedRows` | SpecArch | the records of a one-to-many relation edited as rows under a form |
| `accessibility`, `standard`, `level` | WCAG 2.2 | the accessibility the user interface conforms to |
| `theme`, `tokens`, `$type`, `$value`, `$description`, `colorSpace`, `components`, `alpha`, `hex`, `unit` | W3C Design Tokens Community Group, Format Module 2025.10 | the visual design as design tokens |
| `modes`, `pairs`, `text`, `background`, `use` | SpecArch | the values a mode gives tokens, and the colours shown together, whose contrast is checked |
| `algorithms`, `inputs`, `output`, `formula`, `examples` (of an algorithm), `pseudocode` | SpecArch | IEEE 1016 algorithm viewpoint, made testable |
| `decisions` and the ADR fields | SpecArch, after Michael Nygard's record | the common ADR shape: context, decision, consequences, plus `why` |
| `tests`, `scenario`, `level`, `given`, `when`, `then`, `covers`, `notApplicable` | SpecArch, after ISO/IEC/IEEE 29119 and Gherkin | design tests; the levels are 29119-1's; given, when and then are the Gherkin words, without Gherkin's file format |
| `environments`, `promotesTo`, `configuration`, `secret`, `release`, `rollback`, `migrations`, `steps`, `action`, `check` | SpecArch, after ISO/IEC/IEEE 12207 6.4.10 | the deployment stage: the transition process, written as environments, settings and steps |
| `checks`, `signoff`, `criteria`, `signers` | SpecArch, after ISO/IEC/IEEE 12207 6.4.11 and IEC 62381 | the commissioning stage: validation on the installed system, the site acceptance test |
| `monitors`, `objective` | SpecArch, after ISO/IEC/IEEE 12207 6.4.12 | the operation stage: what is watched on the live system |
| `testing`, `suites`, `designTests`, `designTestsOf`, `implementationOnly` | SpecArch | in implementation files: how one stack runs the design tests |
| `stack`, `targets`, `deployments`, `environment` (of a deployment) | SpecArch | in implementation files: the stack, the output folders per target, and where the system really runs |
| `ownedBy` | SpecArch | in implementation files: on a mapping, the stakeholder that owns an element the project describes but does not generate |

### Traceability links

A design or deployment element names the requirements it meets under
`satisfies`: entity, enum, relation, constraint, transition, permission,
role, operation, command, channel, message, page, algorithm, decision,
environment, setting, release, rollback, migration, and an implementation
decision. A test, a commissioning check or a monitor names the ones it
shows to be met under `verifies`. A requirement names the needs it refines under `needs`. A
field, a parameter, a response, an action or a worked example carries no
link; each traces through the object that holds it. Every ID must be a
requirement of the specification, or belong to an external set declared
under `sources` with kind `requirement-set` and its prefix (`requirement`).

The lists are optional, because a specification is written in order and a
project may not have written its requirements down. Once they are written,
the validator warns for every gap: a need no requirement refines, unless
its status is rejected (`need_unrefined`), a requirement without acceptance criteria
(`acceptance_missing`), a requirement nothing satisfies once the
specification has a design (`requirement_unsatisfied`), and a requirement
nothing verifies once it has tests, checks or monitors (`requirement_unverified`).
Chapter 13 of the techspec is the matrix.

### Access control is fail-closed

Every operation, command and page names exactly one `permission`. Leaving it out
is a schema error. The permission `public` is reserved for things open to
everyone and must be written out; there is no default. A role lists the
permissions it grants and cannot be empty. Row-level rules (a member sees only
their own loans) are not in the meta-model yet; they are described in prose and
enforced by the service, and are on the list in `docs/roadmap.md`.

### Separation of duties

`separationOfDuties` holds sets of permissions one holder must never have
together, after the static separation of duty of the ANSI RBAC standard
(INCITS 359), which states it as a set and a cardinality: no one holds
that many of the set at once.

    separationOfDuties:
      lend-and-write-off:
        description: Whoever lends a copy must not be able to write the loss off.
        permissions: [loans.create, loans.writeoff]
        cardinality: 2

A set names two or more declared permissions, none of them `public`.
`cardinality` is how many of them one holder may not reach: at least 2, at
most the number the set names, and 2 when left out. A role that grants
`cardinality` or more of a set is refused (`separation_of_duties`). The
validator cannot see one person holding two roles that each grant part of
a set, because who holds which role is data; the techspec lists, for each
set, the combinations of roles that together reach it, as roles never to be
given to one person.

### Sessions

A specification may declare how a signed-in session ends:

    session:
      idleTimeout: PT30M
      absoluteTimeout: PT12H

`idleTimeout` is the time without a request after which the session expires,
`absoluteTimeout` the time after signing in after which it expires whatever
the caller does; at least one is given, and neither is zero (`session`).
The two are the OWASP Session Management Cheat Sheet's, which asks for
both. Once a session is declared, every operation, command and page whose
permission is not `public` gets the derived red case `denied with expired
session`, so that a session that outlives its limit is a test, not a hope.

### Dependencies

An external system the operations call is declared once, with the time
limit of one call, and an operation names the ones it calls:

    dependencies:
      feeLedger:
        description: The finance system's ledger, where a late fee is posted.
        timeout: PT5S

    post:
      operationId: returnLoan
      calls: [feeLedger]

A `calls` entry must name a declared dependency, and a `timeout` is never
zero (`dependency`). The limit is on the dependency, not on the call site,
so that one system has one limit wherever it is called. An operation that
calls a dependency gets two derived red cases, `dependency fails <name>`
and `dependency times out <name>`, both critical whatever the operation's
harm, because they are the cases nobody exercises by hand; the expected
outcome is the operation's 502, 503 or 504 response when it declares one.

### Idempotency

GET, PUT and DELETE are idempotent by RFC 9110 (9.2.2): the same request
twice has the effect of once. A POST or PATCH is not, and a client that
loses the answer cannot safely retry it. An operation makes a retry safe by
naming a header parameter as its idempotency key:

    post:
      operationId: createLoan
      idempotencyKey: Idempotency-Key
      parameters:
        - { name: Idempotency-Key, in: header, required: true, schema: { type: string, format: uuid } }

A request repeated with the same key is answered as the first was and has
no second effect; a different request with a key already used is refused.
The key must be a header parameter of the operation (`in: header`), and the
keyword is refused on a method RFC 9110 already makes idempotent
(`idempotency_key`). The header's name follows the IETF HTTP API working
group's Idempotency-Key draft; the project chooses it. The derived cases
are the golden `repeated with the same <key>` and the red `<key> reused
for another request`.

### Validity

An entity whose records are current for a period names the fields that
bound it:

    Member:
      validity: { from: joinedOn, until: membershipEndsOn }

`until` is the date or instant after which a record is expired; `from`,
which may be left out, the one before which it is not yet valid. Both are
fields of the entity, of one format, `date` or `date-time` (`validity`).
Validity sits on the entity, not on a field, because a record is current or
not as a whole. The derivation uses it where a record is taken as input:
an operation whose body names such a record through a relation of the
entity it returns gets the red cases `expired <field>` and, with `from`,
`not yet valid <field>`.

### Guards

An operation or a command that changes data may carry a guard: the entity
it writes, a precondition over that entity's fields that must hold on each
record as it is at the moment of the change, and the exact number of
records it changes:

    post:
      operationId: returnLoan
      guard: { entity: Loan, precondition: 'status == "open" || status == "overdue"', recordsChanged: 1 }

The checks run with the change itself, so a writer who read a record before
another writer changed it is refused rather than overwriting the other's
change, and a change that would touch more records than expected is refused
as a whole. The entity must exist (`guard`), and the precondition is checked
as a check constraint of the entity is: it parses, names the entity's
fields, and gives true or false (`expression_syntax`, `expression_name`,
`expression_type`). The derived cases are `guard precondition fails`, when
there is a precondition, and `concurrent write`, which is critical whatever
the subject's harm. The guard is also what the guarded operational scripts
of `docs/generators.md` are emitted from.

### Sensitive data, encryption at rest, audit fields and soft delete

A field says how sensitive its value is, and whether it is stored
encrypted:

    email: { type: string, format: email, sensitivity: personal, atRest: encrypted, lookup: hash }

`sensitivity` is `public`, `internal` (for staff only), `personal` (about a
person: masked in logs, shown only to who may see it) or `credential` (a
password, a token). A response that can carry a credential, through a
`$ref` to its entity or inline, is refused unless the field is `writeOnly`,
JSON Schema's word for a value that is sent and never returned; a personal
field in the response of a public operation is warned about
(`sensitivity_exposed`). Request bodies are not checked, since a credential
arrives in one by design.

`atRest: encrypted` stores the value encrypted, so it cannot be compared in
storage. `lookup: hash` keeps a salted hash beside it, so it can still be a
key, unique, or found by equality. An encrypted field in a primary key or a
unique constraint needs the lookup, and a lookup on a field that is not
encrypted is refused (`at_rest`).

An entity may be `audited: true`: every record carries `createdAt`,
`createdBy`, `lastModifiedAt` and `lastModifiedBy`, set by the system and
never by a caller. With `deletion: soft`, a delete sets a `deleted` flag and
keeps the record; a deleted record is not listed and reads as not found.
The entity does not declare those fields, since the keyword already says
them (`audited`). The derivation gives a list of such an entity the golden
case `deleted <Entity> not listed`, and a read of one by id the red case
`deleted <Entity> read`. How each is done on a stack (the mask rules, the
engine's encryption and key, the column names) is an idiom
(`docs/idioms.md`): pii-in-logs, encrypted-column, audit-fields and
soft-delete.

### Lists, limits and problem types

An operation that answers a page of an entity's records says so, with the
fields a client may search, filter and sort by:

    listOf:
      entity: Loan
      filterable: [status, memberId]
      sortable: [dueOn, loanedAt]
      pageSize: { default: 20, maximum: 100 }

A request outside the lists, or for a page above the maximum, is refused,
never ignored, since an ignored filter answers rows the caller did not ask
for. The fields must be the entity's; an encrypted field is never
searchable or sortable, and filterable only with `lookup: hash`; the default
page fits the maximum (`list_of`). The parameter names and the answer's
envelope are the paginated-list idiom.

`limits` says the largest request body, `maxRequestBytes`, and a `rate`: so
many `requests` `per` a duration, with a `burst` of at least as many
(`limits`).

`errors` is the catalogue of problem types, in the shape of RFC 9457: each,
keyed by a kebab-case name, has its HTTP `status`, a short `title` that
does not change between occurrences, and the `condition` under which it is
answered. A response names its type under `problem`, of its own status.
Once the catalogue exists, every 4xx and 5xx response names one
(`problem`), so a client can branch on the type without parsing a message.

### Jobs and menus

Work the system does on its own is a job, under `jobs`:

    markOverdue:
      description: Marks open loans past their due date overdue, and tells the member.
      trigger: { schedule: "0 2 * * *" }
      role: scheduler
      reads: [Loan]
      writes: [Loan]
      emits: [loan.overdue/LoanOverdue]
      retries: { limit: 3, then: deadLetter }

The `trigger` is exactly one of `schedule`, the five fields of cron in UTC,
`every`, a duration, or `consumes`, a `channel/Message`. The job acts as a
`role`; it may also `calls` dependencies. The role, the entities and the
consumed message must exist (`job`); emitted messages and called
dependencies are checked as an operation's are (`emits`, `dependency`). A
test names a job as its subject with `job: <name>`. A job gets the cases
`runs twice`, critical on its own, the dependency cases for what it calls,
and `an item fails every try` when it retries.

`menus` is the navigation, a tree of entries, each a `title` with a `page`
or with `items` of its own. Every page an entry opens must exist (`menu`).
An entry is shown to who may open its page.

### Workflows

A request that finishes later, after people approve it, is a workflow,
under `workflows`, keyed by kebab-case name:

    fee-waiver:
      description: A late fee is waived only after a desk supervisor approves it.
      trigger: requestFeeWaiver
      subject: FeeWaiverRequest
      steps:
        - name: approve
          kind: approval
          approvers: [desk-supervisor]
          permission: fees.approve
          deadline: P3D
          onDeadline: refuse
        - name: waive
          kind: operation
          operation: waiveFee

The steps take their meaning from BPMN 2.0, its sequential subset: an
`approval` is a user task whose `approvers` are its potential owners, its
`deadline` a timer on it, and an `operation` step a service task. The
steps run in order, and a refusal at an approval ends the request. The
`trigger` is the operation that starts the workflow; its request body is
the workflow's form, and it answers 202, accepted and not yet done
(RFC 9110). The `subject` is the entity that holds the request while it
waits. An approval names the roles that may approve, the `permission` it
checks, a `deadline`, a duration above zero, and `onDeadline`: `refuse`
ends the request, `escalate` moves it to the later approval named under
`escalateTo`. An operation step names the operation the system calls once
every approval before it has passed.

The person who made a request never approves it. The rule holds on every
approval and has no switch; whether two people are one is known only when
the workflow runs, so it is a derived case, not a check.

The validator refuses a trigger that is not an operation or does not
answer 202, a subject that is not an entity, an approver that is not a
role or does not grant the approval's permission, an approval that checks
the trigger's permission, a role that grants both the trigger's and an
approval's permission while a separation-of-duties set holds the pair,
whatever the set's cardinality, a deadline of zero, an operation step naming no
operation, an escalation to a step that is not a later approval, and two
steps of one name (`workflow`); an approval's permission must be declared
(`permission_undeclared`). A test names a workflow as its subject with
`workflow: <name>`. Parallel approvals, a number of approvals out of a
pool and loops are outside the subset; a workflow that needs one is
written with a question.

### Maker-checker on a page

A form or task page that submits to a workflow's trigger makes the
request, and its submit is answered 202: accepted, and waiting for
approval. The page acts on that answer, a form with its `onSubmitted` and
a task under `onSubmitted."202"`, and the event carries the message the
person reads, which is the page's pending state:

    fee-waiver-form:
      kind: form
      entity: FeeWaiverRequest
      submit: requestFeeWaiver
      onSubmitted: { navigate: loans-list, message: Sent for approval. A desk supervisor answers within three days. }

A list page may be the inbox of an approval step, listing the requests
that wait there:

    fee-waivers-inbox:
      kind: list
      entity: FeeWaiverRequest
      permission: fees.approve
      source: listFeeWaivers
      inbox: { workflow: fee-waiver, step: approve }

The inbox lists the workflow's `subject` and checks the step's
`permission`, so it opens to whoever may approve and to nobody else. A
workflow names the channel messages it publishes when it ends, approved or
refused, under `emits`, as an operation does, so a page waiting on it
knows how the end is announced.

The validator refuses a page submitting to a trigger with no event for
the 202 answer or no message in it, and an inbox on a page that is not a
list, naming no workflow or no approval step of it, listing another entity
than the subject or checking another permission than the step's
(`workflow`); a workflow's message that no channel declares is `emits`. A
form that makes a request gets the derived case `sent for approval`, its
202 answer with the message shown; a task page has it as its case for
the 202 answer. Someone without the step's permission is refused the
inbox by the page's own `denied` case, so it lists nothing to them.

### Page events

A page raises events: a form is submitted, a row of a list is selected,
an operation run from an action succeeds. Each says where it leads:

    member-form:
      kind: form
      submit: createMember
      onSubmitted: { navigate: member-view, with: { memberId: id }, message: The member is registered. }
    members-list:
      kind: list
      onSelect: { navigate: member-view, with: { memberId: id } }
      actions:
        - { label: Remove, kind: operation, target: removeMember, then: { message: The member is removed. } }

`onSubmitted` is a form's, `onSelect` a list's, and `then` an action's of
kind operation; an action of kind navigate leads already. `navigate` names
a page, and `with` gives exactly that page's route parameters, each from
a field of the page's entity: the record submitted, selected or acted on.
`message` is a status message, a full sentence, announced without moving
focus (WCAG 2.2, 4.1.3). A full sentence starts with a capital, a letter
of a script without case or a digit, and ends with a full stop, a
question mark or an exclamation mark, in Latin or CJK form. An event with only a message stays on the page.
Every rule here is `flow`. The concepts are IFML's events and navigation
flows; its diagram notation is not used, and the techspec draws the
screen flow from the pages.

A flow is a task a person does across pages, under `flows`, keyed in
kebab case:

    lend-a-copy:
      description: A librarian finds the member at the desk, opens their record and lends them a copy.
      actor: librarian
      steps:
        - { page: members-list, event: select }
        - { page: member-view, event: action, action: Lend a book }
        - { page: loan-form, event: submitted }

The `actor` is a role, and it must be allowed to open every page on the
way and to take every action a step names. Each step is a page and the `event` on it: `select`, `submitted`, or
`action` with the action's label. Every event but the last must lead to
the next step's page (`flow`). The techspec draws each flow as its steps.
A test names a flow as its subject with `flow: <name>`; its golden case
walks the steps.

### Task pages

A page of kind `task` submits to an operation without loading a record:
sign-in, a second factor, a password reset, a confirmation of an e-mail
address.

    sign-in:
      kind: task
      title: Sign in
      route: /sign-in
      permission: public
      submit: signIn
      fields: [email, password]
      onSubmitted:
        "200": { navigate: home }
        "202": { navigate: second-factor, with: { challengeId: challengeId } }

A task page has no `entity`, `source`, `columns` or `filters`. Its
`fields`, or the fields of its `sections`, are properties of the submit
operation's request body, and every property the body requires is among
them; an operation that takes no body cannot be a task's (`page`). So the
rules a person sees are the body's, written once, in the operation.

`onSubmitted` is keyed by the success status the page acts on, each one
the operation declares (`page`), and each event is a page event as above,
whose `with` takes the route parameters of the page it leads to from
properties of that response's body (`flow`). A problem the operation
answers is shown under the page's failed states, which may name the field
it is about. In a flow, a `submitted` step on a task page leads on when
any of its answers leads to the next step's page.

The derived cases are one per answer: the first success the operation
declares is the golden case, each other success is a golden case
`answered <status>`, and each problem type is a red case `fails with
<problem>`. A public task page has no denied cases and no expired
session. The techspec draws each answer as an edge labelled with its
status.

### Page states

A page may say what it shows when it is empty or fails:

    loans-list:
      kind: list
      filters: [status, memberId]
      states:
        empty: { message: No loans yet. A loan is made from a member's record. }
        filteredEmpty: { message: No loan matches these filters. }
        failed:
          loan-closed: { message: "This loan was already closed, so nothing changed." }
          default: { message: The loans cannot be shown or changed right now. Try again in a moment. }
    member-form:
      kind: form
      states:
        failed:
          email-taken: { message: Another member already has this email address., field: email }

`empty` is a list's, and `filteredEmpty` a list's with filters. `failed`
is keyed by the problem types the page's operations can answer: the one
it reads or submits, and those its actions run; `default` covers every
problem type not named. A form's failed state may name the `field` it is
about, so the message shows beside it (WCAG 2.2, 3.3.1). Every message
is a full sentence. Loading and submitting have no text: the stack draws
them, and offers to retry a failed read and to clear filters the same way
on every page.

A page without `states` shows what its stack shows. Once a page has
`states`, they are complete: a list has `empty`, a list with filters has
`filteredEmpty`, every problem type has a message or there is a
`default`, and nothing else is named (`state`). Each state is a derived
case of the page: `empty`, `filtered empty`, and `fails with <problem>`.

### Sections

A form or a view gives its fields once: as `fields`, or as `sections`,
each a `title` and its `fields`, in the order a person reads them, which
is the focus order (WCAG 2.2, 2.4.3):

    member-view:
      kind: view
      sections:
        - { title: Member, fields: [cardNumber, fullName, email, tier] }
        - { title: Membership, fields: [joinedOn, membershipEndsOn, outstandingFees] }

A page with neither or both, a field in two sections, and sections on a
list are refused (`page`). Where the sections sit on a screen is the
stack's.

### Compact screens

A list says which of its columns a compact screen keeps, in order:

    loans-list:
      columns: [memberId, bookId, loanedAt, dueOn, status, lateFee]
      compactColumns: [memberId, dueOn, status]

Each must be one of the list's columns, and only a list has them
(`page`). How small compact is, and how a compact row is laid out, are the
stack's.

### Child rows

A form edits the records of a one-to-many relation of its entity as rows
under it, each with a `title`, the `fields` of the relation's target shown
in a row, the `maximum` number of rows, loaded and new together, and
`lockLoadedRows: true` when the rows loaded with the record cannot be
changed or removed, so only new ones change:

    member-loans:
      kind: form
      entity: Member
      childRows:
        - { relation: loans, title: Loans, fields: [bookId, dueOn], maximum: 6, lockLoadedRows: true }

Each row is validated by the target entity's own schema. Child rows on a
page that is not a form, a relation named twice, and a relation the
form's entity lacks or that is not one-to-many are refused (`page`); a
field the target lacks is refused (`field`). A many-to-one or one-to-one
relation holds one record, not rows, and the rows of a many-to-many
relation are records of its join entity. A maximum gives the page the
derived case `<relation> with more than <maximum> rows`. Whether rows are
edited inline or in a dialog is the stack's.

### Page elements

A form picks a field that holds another record's key from a list of
those records, says when a field is read-only or hidden, checks rules
across its fields, and asks for a field twice; a task checks and asks
twice as a form does; an action is offered only in some states and may
ask for a reason:

    loan-form:
      kind: form
      entity: Loan
      fields: [memberId, bookId, lentOn, dueOn, pin]
      pickers:
        memberId: { source: listMembers, shows: [cardNumber, fullName] }
      fieldConditions:
        dueOn: { readOnlyWhen: 'lentOn == dueOn' }
      checks:
        due-after-lent: { expression: dueOn > lentOn, message: A copy is due after the day it is lent., field: dueOn }
      enteredTwice: [pin]
    members-list:
      kind: list
      actions:
        - { label: Deactivate, kind: operation, target: deactivateMember, when: 'status == "active"', confirm: "Deactivate this member?", reason: reason }

- `pickers` are a form's, keyed by a field it shows. The record's entity
  is the target of the many-to-one relation of the page's entity whose
  `via` is the field; `source` is an operation whose `listOf` names that
  target; `shows` are the target's fields a person reads to choose;
  `fills` sets other fields of the form from the chosen record, each
  from a field of the same type. Every role that may open the form may
  also call the source (`picker`).
- `when` on an action is an expression over the record it acts on: a
  row of a list, or the record a view or an edit form shows. A form
  without `source` creates a record, so its actions have no `when`.
- `reason` on an action that runs an operation and has `confirm` names
  the property of the operation's request body that carries the reason
  typed in the confirmation, a string the body requires (`action`).
- `fieldConditions` are a form's or a view's, keyed by a field it shows:
  `readOnly`, `readOnlyWhen` and `hiddenWhen`. Every field of a view is
  read-only already. A page is one mode: a form without `source`
  creates, a form with one edits, and a view shows; a field hidden in a
  mode is one that page does not list.
- `checks` are a form's or a task's, keyed in kebab case, each an
  `expression` over the fields it shows, a `message` in a full sentence
  and the `field` it shows beside. `enteredTwice` names fields a person
  types twice; the second entry is compared and never sent. On a task
  the fields are the properties of the request body it sends, so a
  confirmation the operation takes is checked against the password
  before it is sent:

      reset-password:
        kind: task
        submit: resetPassword
        fields: [code, newPassword, confirmPassword]
        checks:
          confirmation-matches: { expression: confirmPassword == newPassword, message: The two passwords are not the same., field: confirmPassword }

Each expression is in the subset under Expressions and gives true or
false, never null. On a form that creates a record it names the fields
the form shows, on a task the properties of its request body it shows,
and elsewhere the entity's fields. The misuses are
`picker`, `action` and `form_field`; an expression that does not parse,
names what it cannot or has the wrong type is an expression error. The
techspec lists every element of every page.

### Accessibility

The specification names its target once, in the design:

    accessibility: { standard: WCAG 2.2, level: AA }

With it, every field a page shows or filters by has a `title`, JSON
Schema's keyword, which is the label a person reads beside it (WCAG 2.2,
3.3.2 and 2.4.6), and no two actions of a page share a label (4.1.2);
both are `accessibility`. Without it, neither is checked; the theme's
contrast is checked either way. The techspec lists the criteria of the
level that the design settles or leaves to the generator, and says who
meets each; every other criterion is checked at commissioning.

### Theme

The visual design is one object, `theme`, in `design/theme.yaml`. Its
`tokens` are in the format of the W3C Design Tokens Community Group:
groups and tokens, each token a `$value` and a `$type` given by it or a
group above it, an alias of another token written `{group.token}`, which
takes that token's type when nothing else gives one, and the format's
`$description`, `$extensions` and `$deprecated`, which SpecArch keeps and
does not read:

    theme:
      tokens:
        color:
          $type: color
          text: { $value: { colorSpace: srgb, components: [0.1059, 0.1216, 0.1412], hex: "#1b1f24" } }
          background: { $value: { colorSpace: srgb, components: [1, 1, 1], hex: "#ffffff" } }
          accent: { $value: { colorSpace: srgb, components: [0.0431, 0.3608, 0.6784], hex: "#0b5cad" } }
          link: { $value: "{color.accent}" }
        space:
          $type: dimension
          small: { $value: { value: 8, unit: px } }
      modes:
        dark:
          color.text: { colorSpace: srgb, components: [0.902, 0.9098, 0.9216], hex: "#e6e8eb" }
          color.background: { colorSpace: srgb, components: [0.0627, 0.0706, 0.0784], hex: "#101214" }
          color.accent: { colorSpace: srgb, components: [0.4235, 0.6902, 0.9608], hex: "#6cb0f5" }
      pairs:
        - { text: color.text, background: color.background, use: text }
        - { text: color.link, background: color.background, use: text }

The types taken are color (in the srgb space, three components from 0 to
1, and an `alpha` and a `hex` that must be the components' colour),
dimension (px or rem), fontFamily, fontWeight, duration (ms or s) and
number. A number is written in decimals: a hexadecimal, octal or binary
integer and a digit separator are refused. `modes` gives tokens, by path, another value; the tokens' own
values are the default. Each pair is a `text` colour on a `background`
colour, with its `use`: `text`, `largeText` (18 point, or 14 point bold)
or `control` (the parts of a control and graphics). Its contrast must
reach what WCAG 2.2 asks of the use, at AAA when the accessibility target
is AAA and at AA otherwise, since WCAG asks no contrast at level A, in
every mode: 4.5:1 for text, 3:1 for large text
and controls, and at AAA 7:1 and 4.5:1 for text (1.4.3, 1.4.6, 1.4.11).
A translucent colour in a pair is refused, since its contrast depends on
what lies beneath. Every rule here is `theme`. How a token becomes code is
the stack's.

### Views

A list page shows a row of one entity with names and counts joined onto
it. That row is a view, under `views`, named like an entity:

    LoanRow:
      description: A loan as the desk's list shows it.
      from: Loan
      properties:
        memberName: { path: member.fullName }
        branchName: { path: member.branch.name }
    MemberRow:
      from: Member
      properties:
        openLoans: { count: loans }
    MemberWithLoans:
      from: Member
      properties:
        loans: { rows: loans }

A view holds one row per record of its `from` entity and carries every
field of it, the audit and deleted fields included, so its key still
names one record. Each property it adds is exactly one of `path`,
relations separated by dots that each lead to one record (many-to-one or
one-to-one) and end in a field; `count`, a relation of the entity that
leads to many (one-to-many or many-to-many); or `rows`, a one-to-many
relation of the entity whose records the view carries. A path has its
field's type and is null when a relation on it has no record; a count is a
64-bit integer and leaves out softly deleted records; rows are a list of
the relation's target, empty when it has none, softly deleted records
left out. The rows of a many-to-many relation are records of its join
entity, so they are carried through the entity's one-to-many relation to
the join entity. A property may not repeat a field of the entity, and a
view may not share an entity's name (`view`).

Rows are how a record is read with the records of a relation, as a form
that edits them as child rows loads it. A list holds one row per record,
so a view with rows is never a list's subject (`list_of`), and rows are no
column in SQL: a SQL view holds one row of plain columns per record, the
rows are the related table's own, read through the relation's key, and a
view that adds only rows has no SQL view at all.

A view is never written. It may be the item of a response, as
`$ref: "#/views/LoanRow"`, and the subject of a list, as
`listOf: { view: LoanRow, ... }`, whose whitelists then name the view's
fields; under a request body it is refused (`view`). The SQL standard lets
a view be updated only when it reads one table without grouping, and the
engines differ beyond that, so a write goes to the entity, its one place.
In SQL a view is named as a table is: the view's name in snake case,
unless the implementation file's mapping names it (`target: view
loan_rows`).

### Value objects

Data passed around but not stored, with no identity of its own (a
diagnostic, a request's summary, a token's claims), is a schema under
`schemas`, named as OpenAPI names the shapes under `components.schemas`:

    Diagnostic:
      description: One thing a check found, passed back and never kept.
      type: object
      properties:
        severity: { $ref: "#/enums/Severity" }
        text: { type: string, maxLength: 200 }
      required: [severity, text]

A schema has `type: object`, `properties` and `required`, and nothing only
a stored record has: no primary key, relations, constraints, states,
validity, audit or deletion. A request body, a response, a message's
payload and another schema's property refer to it as
`$ref: "#/schemas/Diagnostic"`, and a derived case reads a request body's
required properties through it as it does through an entity. An entity
may not relate to a schema, since a relation leads to a record and a
schema has none; a schema may not share the name of an entity or a view,
since all three become schemas of one interface (`value_object`).
`generate sql` writes no table for a schema; `generate openapi` writes it
under `components.schemas` as it is.

An entity's field may hold a schema, or a list of them, as a part of the
entity's record (ADR-063):

    Member:
      properties:
        address: { $ref: "#/schemas/Address" }
        preferences: { $ref: "#/schemas/Preferences", storage: json }
        phones: { type: array, items: { $ref: "#/schemas/Phone" } }

`storage` says how the value is kept in the entity's row. `columns`, the
default for one value, gives each part a column named after the path to
it: `address_street`, `address_location_latitude` for a schema inside the
address. A part's column is NOT NULL only where the part and every value
above it are required, and an optional value gets a check that it is
either wholly absent or has every required part. `json` keeps the whole
value in one column of the dialect's JSON type, and is the only storage
for a list: a list that must be queried row by row has an identity, so it
is an entity related to its owner. In columns a value may not hold a list
of schemas, itself at any depth, or a reference to an entity, and an
optional one needs a required part that is never null, or a row without
it could not be told from one where every part is empty; `storage` goes only on an
entity's own field that holds a schema. Each is refused as
`value_object`. In the interface the field is a reference to the schema
whatever its storage.

A specification read from sources finds the value again (ADR-077). `extract
openapi` writes an entity's property that refers to a component schema as a
reference to it, with `storage: json` where columns would be refused, and
`extract database` writes a JSON column as a field of type `object` with a
question asking which schema it holds. `specarch merge` writes each field
holding a schema out as `generate sql` does and reads the code side's
columns back as that field where they are exactly what it writes: every
part's column, with its type, width and nullability, and the check that an
optional value is wholly absent or has its required parts. Where they
differ it asks at the field, and where no tree names a field but an
entity's columns are every part of a schema under one prefix, it asks
whether they are one value.

### Secrets

A setting under `configuration` says whether it is a `secret`. A secret's
value is never written anywhere in a specification or an implementation
file: not as a `default`, not under a deployment's `configuration`. The
validator refuses both (`secret_value`). The setting's description says
where the value comes from, such as the host's secret store.

### Settings that switch an element on

An operation, a command or a page served only while a setting is on names
it under `enabledBy`. The setting is a boolean of `configuration`; the
validator refuses a name that is not a setting, or a setting of another
type (`setting`), since a switch has two positions and anything else would
need a comparison written somewhere. With the setting off the element is
refused as not available, and the derived case `disabled by <setting>`
asks for a test of it.

    paths:
      /exports:
        post:
          operationId: startExport
          enabledBy: exportsOn

### Expressions

`check` constraints and `formula` strings are written in a small subset of
CEL, the Common Expression Language (cel.dev). Its syntax is the one C, Java
and JavaScript use, so most readers already know it, and its type rules are
strict: values of different types never mix without a written conversion.
CEL is strict on purpose. An implicit conversion is where precision is lost
without anyone seeing it (an int64 turned into a double, a decimal into a
float), so every conversion is written where it happens.

A check is one expression that gives a bool, and so is the `where` of a
unique constraint: the fields must be unique only among the records it
holds for. A formula is one expression
whose type is the algorithm's output type. The validator parses every
expression, refuses anything outside the subset by name, checks every name
and type against the declared fields and inputs, and evaluates every formula
on its worked examples.

Types in expressions are CEL's: `int` (every integer field, int32 included,
is an int; the field's width is checked when a value is computed), `uint`,
`double`, `string`, `bool`, `bytes`, `timestamp` and `duration`, plus three
that CEL does not have and SpecArch adds: `decimal`, `date` and enum values.
CEL has no decimal because it was built for protocol buffers, which have
none; money needs one.

| Part | Example | Meaning |
|---|---|---|
| int | `21`, `-3` | a 64-bit integer |
| uint | `21u` | an unsigned 64-bit integer |
| double | `0.5`, `2.0` | a 64-bit float; `2` is an int and `2.0` a double |
| string | `"open"`, `'open'` | text, in double or single quotes |
| bool | `true`, `false` | |
| `null` | `returnedAt == null` | no value; only for a field that allows null |
| name | `dueOn` | a field of the entity (in a check) or an input of the algorithm (in a formula) |
| `( )` | `(a + b) * c` | grouping |
| `-x` | `-discount` | negation |
| `!x` | `!active` | not |
| `*`, `/`, `+`, `-` | `subtotal + tax` | arithmetic on two values of the same numeric type |
| `==`, `!=` | `status == "open"` | equal, not equal; same type on both sides |
| `<`, `<=`, `>`, `>=` | `copiesAvailable <= copiesOwned` | order; same type on both sides |
| `&&`, `\|\|` | `returnedAt == null \|\| returnedAt >= loanedAt` | and, or |
| `c ? a : b` | `daysLate > 0 ? 1 : 0` | `a` when `c` holds, otherwise `b`; both of one type |
| `int(x)` | `int(round(total, 0))` | to int, from uint, a string of digits, or a rounded decimal or double |
| `uint(x)` | `uint(count)` | to uint, from int or a string of digits |
| `double(x)` | `double(count)` | to double, from int, uint or decimal; the result is approximate |
| `decimal(x, scale)` | `decimal(daysLate, 0)`, `decimal("0.50", 2)` | to decimal with that scale, from int, uint, a quoted number, or a decimal of no larger scale |
| `string(x)` | `string(count)` | to string |
| `date(x)` | `dueOn > date(loanedAt)` | the date of a timestamp, or a quoted date: `date("2026-10-07")` |
| `timestamp(x)` | `timestamp("2026-10-07T09:30:00Z")` | a quoted RFC 3339 instant |
| `duration(x)` | `duration("P14D")`, `duration("PT2H")` | a quoted ISO 8601 duration in days, hours, minutes and seconds |
| date `+`, `-` duration | `dueOn == loanedOn + duration("P14D")` | a date moved by whole days, the date first and the days written in place |
| `size(x)` | `size(fullName) <= 200` | the length of a string or a list, an int |
| `round(x, places)` | `round(total, 2)` | a decimal or double rounded half away from zero; a decimal result has that scale |
| `floor(x)`, `ceil(x)` | `floor(hours)` | down or up to a whole number, keeping the type |
| `min(a, b, ...)`, `max(a, b, ...)` | `min(fee, replacementCost)` | the smallest or largest of two or more values of one type |

A date moves only by whole days written in place: `loanedOn + 14` is
refused, since a number names no unit, and so is `duration("PT12H")`
added to a date, since a date has no time of day. `duration` reads the
ISO 8601 form every other duration of the meta-model is written in, not
CEL's `"336h"`, which has no day because CEL has no date.

Decimal scales are tracked. `+` and `-` give the larger of the two scales,
`*` the sum of the scales, `min`, `max` and `? :` the larger; `/` gives an
exact result of no fixed scale, so it has to be rounded before it is
returned. A formula's decimal result must have no more places than the
output's `scale`: write `round(x, 2)` where the design wants rounding. A
conversion that would lose information is refused unless the rounding is
written: `int(round(x, 0))`, `decimal(round(x, 2), 2)`. A double cannot be
turned into a decimal at all, because the digits it lost cannot be brought
back.

Nothing else is in the subset: no `%`, no `in`, no lists or maps written in
the expression, no field access with `.`, no macros (`has`, `all`,
`exists`, `map`, `filter`), and no other function. Each is refused with a
message naming it. Comparisons do not chain: `a < b < c` is an error,
written `a < b && b < c`. CEL itself lets numbers of different types be
compared; SpecArch does not, so that every operator follows one rule, and a
mixed comparison is written with a conversion. Anything a real
specification needs beyond this list is added on purpose, with an example
here.

A worked example's values have the declared types: an int32 or int64 is a
YAML integer, an int64 carried as text a quoted string of digits, a decimal
a quoted string (`"0.50"`) with no more places than its scale, a date a
quoted date, a timestamp a quoted RFC 3339 instant.

### Tests

Tests are part of the specification, split along the same line as
everything else. The specification's `tests` say what must hold on every
stack; an implementation file's `testing` says how one stack runs them, plus
the tests of its own code (unit and integration suites, fixtures, mocks,
performance targets, platforms).

A design test is about one subject: an `operation`, a `command`, a `page`,
a `job`, a `flow`, a `workflow`, a `requirement`, or an `entity`, with one of its `constraint`s or
`transition`s or alone for its state machine. It has a
`level`, `system` (the test exercises the system through its interfaces as a
client would) or `acceptance` (it shows a stakeholder that a requirement is
met, and names it under `verifies`); unit and integration tests belong to
the implementation's suites, which carry the same `level` key. It is marked
`scenario: golden` for the path that succeeds or `scenario: red` for a path
that fails, and says what happens in three plain sentences: `given`, `when`
and `then`. For a generator a test may add `fixture` (the caller and the
records that exist), `input` (what the call carries) and `expect` (the
status, exit, body, state and messages after), in the design's own
vocabulary and checked against it (`test_data`; `docs/test-generation.md`
gives the shapes). There is no test language beyond that. In a tree, each
test is `tests/<name>/test.yaml`, with the scenario's own data files beside
it, for what the structured keys cannot hold; a test says one thing one
way, so a test with `input` has no `input/` folder and one with `expect`
no `expected/` folder:

    tests/lend-limit-reached/test.yaml
    tests/lend-limit-reached/input/request.json
    tests/lend-limit-reached/expected/response.json

    operation: createLoan
    level: acceptance
    scenario: red
    covers: [response 409]
    given: a standard-tier member with three open loans
    when: createLoan is called for a fourth book
    then: it answers 409 and no loan is created
    verifies: [LIB-3]

Every operation, command, page, constraint and transition needs at least
one golden scenario and its red ones; any golden test of it is its success
case, and the test the validator suggests when there is none carries the
outcome the design gives (the 2xx response, exit 0, the page shown, the
state reached). A requirement and a state machine are subjects of a
different kind: every case is golden and asked for on its own.

A suggested test is named after its subject, then its case:
`create-loan-missing-member-id`. When subjects of different kinds give
the same name, as page `sign-in` and operation `signIn` both give
`sign-in`, each takes its subject key in front: `page-sign-in-succeeds`
and `operation-sign-in-succeeds`.

- A `requirement` subject has one case per acceptance criterion,
  `acceptance 1`, `acceptance 2` and so on, with the criterion as its
  outcome; its tests are usually `level: acceptance`.
- An `entity` alone is its state machine, declared by `stateField` and
  `transitions`. Its cases are its paths from an initial state (one no
  transition reaches) to a terminal state (one no transition leaves),
  taking the transitions in document order and never visiting a state
  twice, named by their states: `open to overdue to returned`.

The validator derives the cases a subject needs from the rest of the
specification. Each case has a frequency, how often users make that
mistake:

| Found in the design | Case | Scenario | Frequency |
|---|---|---|---|
| a required field of a request body | `missing <field>` | red | frequent |
| `minimum` / `maximum` of a field or parameter | `<field> below minimum N` / `<field> above maximum N` | red | occasional |
| | `<field> at minimum N` / `<field> at maximum N` | golden | occasional |
| `exclusiveMinimum` / `exclusiveMaximum` | `<field> at exclusive minimum N` / `... maximum N` | red | occasional |
| `minLength` / `maxLength` | `<field> shorter than N characters` / `<field> longer than N characters` | red | occasional |
| | `<field> of N characters` | golden | occasional |
| `minItems` / `maxItems` | `<field> with fewer than N items` / `<field> with more than N items` | red | occasional |
| `pattern` | `<field> not matching its pattern` | red | frequent |
| an enum | `<field> not one of its values` | red | frequent |
| a format that values can break (email, uuid, date, decimal...) | `<field> not a valid <format>` | red | frequent |
| a permission other than `public` on an operation, command or page | `denied without <permission>` | red | frequent |
| a path parameter, or a route parameter of a page | `not found <parameter>` | red | frequent |
| a body field that is the `via` of a relation of the entity the operation returns | `not found <field>` | red | frequent |
| a unique constraint of the entity a POST with a 201 response creates | `duplicate <constraint>` | red | occasional |
| a channel the operation `emits` on | `dependency fails <channel>` | red | rare |
| a dependency the operation `calls` | `dependency fails <dependency>`, `dependency times out <dependency>` | red | rare |
| an `idempotencyKey` | `repeated with the same <key>` | golden | frequent |
| | `<key> reused for another request` | red | occasional |
| a body field that is the `via` of a relation to an entity with `validity` | `expired <field>`, and with `from` `not yet valid <field>` | red | occasional |
| a GET answering a list of an entity with `deletion: soft` | `deleted <Entity> not listed` | golden | occasional |
| a GET by a path parameter answering an entity with `deletion: soft` | `deleted <Entity> read` | red | occasional |
| an operation with `listOf` | `page beyond last` | golden | occasional |
| an operation with `listOf` | `page size above <maximum>`; with `sortable`, `sort by a field not sortable`; with `filterable`, `filter by a field not filterable` | red | occasional |
| an operation with `limits.maxRequestBytes` | `request larger than <n> bytes` | red | occasional |
| an operation with `limits.rate` | `rate exceeded` | red | occasional |
| a job | `runs twice` | golden | critical on its own |
| a job that calls a dependency | `dependency fails <name>`, `dependency times out <name>` | red | critical on its own |
| a job with `retries` | `an item fails every try` | red | occasional |
| an approval of a workflow | `refused at <step>`, `approval without <permission>` | red | frequent |
| | `deadline passes at <step>` | red | critical on its own |
| a workflow with an approval | `requester approves own request` | red | critical on its own |
| a `session`, for every permission other than `public` | `denied with expired session` | red | frequent |
| a `guard` with a precondition | `guard precondition fails` | red | occasional |
| a `guard` | `concurrent write` | red | rare |
| a 4xx or 5xx response | `response <status>` | red | occasional |
| a command | `usage error`, and `exit <status>` for each non-zero exit code | red | frequent |
| a check constraint / a unique constraint | `violates <constraint>` / `duplicate <constraint>` | red | occasional |
| a unique constraint with `where` | `duplicate outside the condition` | golden | occasional |
| an operation, command or page with `enabledBy` | `disabled by <setting>` | red | frequent |
| a check constraint that can be false in more than one way | `violates <constraint>: <clause> is false`, one per way (below) | red | occasional |
| an acceptance criterion of a requirement | `acceptance <N>` | golden | occasional |
| a path of a state machine | `<state> to <state> to ...` | golden | occasional |
| a transition | `from wrong state` | red | frequent |
| a picker of a form | `picker <field> finds nothing` | red | occasional |
| an action with `when` | `<label> not offered` | red | occasional |
| an action with `reason` | `<label> without a reason` | red | frequent |
| a check of a form | `violates <check>`, or one per way it can be false | red | frequent |
| a field entered twice | `<field> entered twice differently` | red | frequent |

A check constraint whose expression joins clauses with `&&` or `||` is a
decision table (ISO/IEC/IEEE 29119-4, 5.2.6): one red case per way it can
be false, each clause of an `&&` false alone and every disjunct of an `||`
false at once. The case names the false clauses in the expression's own
text, `violates copies_in_range: copies >= 0 is false`, so that every
build names it alike; a constraint false in only one way keeps the plain
`violates <constraint>`. A case name holding a colon or a quote is quoted
in `covers`.

An acceptance case is chosen when its requirement names a `harm`, and a
path when one of its transitions satisfies a requirement with a harm; the
others are left out, as below.

A field or a parameter's schema may replace the frequency of every case
derived from it (its limits, pattern, enum and format, and `missing` or
`not found` for it) with `mistakes`:

    fullName: { type: string, maxLength: 200, mistakes: rare }
    email: { type: string, format: email, mistakes: frequent }

A requirement may name what is at stake when it is not met, under `harm`:
`data-loss`, `money`, `security`, `safety`, `privacy` or `availability`.
`priority` does not do this: it says whether a release may go without the
requirement, not what its failure costs.

    LIB-3:
      statement: A member shall have at most three open loans.
      harm: [money]

A requirement may also name the release it is meant for, under `release`,
by version. A release record of that version must exist, planned or
released (`record_ref`), so a scope split into a first and a later release
is written in the requirements, and the test plan groups the requirements
by release with the tests that verify each.

Each derived case then has a rank. It is `critical` when its subject
satisfies, under its own `satisfies`, a requirement with a harm, or when
it is a case nobody exercises by hand (a failing or slow dependency, two
writers on one record); `frequent` when its frequency is frequent; and
`other` otherwise. The validator warns once for every critical or frequent
case no test lists under `covers` (`test_case_missing`), with a test to
copy. The cases of rank `other` that no test covers are left out: the test
plan lists them under "Derived cases left out", each with the reason, and
writing a test for one removes it from the list. Once a requirement names
a harm, the traceability matrices have a Harm column.

One test may cover several cases when they are one scenario (a lookup that
answers 404 covers `not found memberId` and `response 404`). A case that
does not apply is covered by a test with `notApplicable` and a written
reason instead of given, when and then. In 0.1 a missing scenario is a
warning, so a specification can be written before its tests; it is meant to
become an error in the next version.

An implementation file's suite names the design tests it runs, by name
(`designTests`) or by subject (`designTestsOf: [{ command: validate }]`),
or says `implementationOnly: true`, and carries its `level`: a suite that
runs design tests is `system` or `acceptance`, an implementation-only suite
`unit` or `integration`. The validator checks every name it gives exists.

### Commissioning records

A commissioning run's results are records, not design: a fact about one
run. They live outside the specification, one file per run, under
`records/commissioning/` beside the specification's folder, named by date
and environment (`records/commissioning/2026-10-07-production.yaml`). Each
holds the `environment`, the `date`, the `version` of the specification and
the build, the `operator` as a role, `results` keyed by check name with
`result` (pass, fail or skipped) and a `note`, and the `signoff` with who
signed and when. The commissioning procedure (`specarch document
commissioning`) is the form a run fills in. A commissioning record is one
kind of record; `docs/maintenance.md` describes them all and the rules the
validator checks them against.

### Approval records

`specarch approve --by <stakeholder>` records that a stakeholder read the
documents of a specification and approves it for code generation, as
`records/approvals/<version>.yaml` beside the specification's folder:

    specarchRecord: "0.1"
    kind: approval
    version: 1.4.0
    approvedBy: product-owner
    date: "2026-10-08"
    documents: [techspec, requirements, testplan]
    digest: sha256:9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08

The record is written only when the specification has no error and no
`must` or `should` question, `--by` is one of its stakeholders, and every
configured document on disk is what the specification generates now. The
digest is the SHA-256 of every `.yaml` file under the specification's
folder, in byte order of their paths relative to it, each as its path, a
zero byte, its bytes and a zero byte; any later change to one of those
files voids the approval. `specarch generate` refuses a target while a
`must` or `should` question blocks a section it reads (the sections the
implementation file names under `targets.<name>.reads`, or every section),
and refuses without an approval whose digest is the digest of the files
now, unless `--unapproved` is given. An earlier approval of the same
version is replaced.

### What the schema cannot check

JSON Schema validates shape: types, required keys, patterns, closed objects.
It does not know that `target: Loan` must name an entity of the
specification, that a `primaryKey` field must exist in `properties`, that a
page's `columns` belong to its `entity`, that a `transition` uses values of
the state field's enum, that a citation's `source` is declared, or that a
check's `environment` exists. Those are cross-reference checks, made by
`specarch validate` together with the layout, expression, worked-example,
access and traceability checks. A tree that passes the schema can still
fail the validator.

## Markdown sections

The `specarch.md` file beside the root file explains the specification. The
sections follow arc42, trimmed to what a small system needs. Each heading is
fixed so a target can find it and so readers of different specifications
know where to look.

1. Introduction and goals: what the system is for, who uses it, the top
   quality goals.
2. Constraints: technical, organisational, legal; the `constraints` and
   `assumptions` of the requirements stage, explained.
3. Context: the system and its neighbours. One hand-drawn context picture.
4. Solution strategy: the few decisions that shape everything else, each
   pointing at its ADR.
5. Building blocks: generated entity diagram, then prose on each entity
   group.
6. Runtime view: generated sequence diagrams for the main operations,
   generated state diagram for each entity with a `stateField`.
7. Deployment: hand-drawn picture; the environments, release and
   commissioning checks are in their stages.
8. Cross-cutting concepts: permissions table (generated), money, time,
   identifiers, error handling.
9. Architecture decisions: generated list from `decisions`, with a link to
   each.
10. Quality requirements: scenarios.
11. Risks and technical debt.
12. Glossary: the `glossary` section, explained where it needs it.
13. Requirements: the requirements and the traceability matrix are in the
    requirements stage and in the techspec; this section says where.

A section with nothing to say is left out; the others keep their numbers,
so section 8 is always cross-cutting concepts. A placeholder line would only
be noise for the reader.

## Mermaid diagrams: generated or hand-drawn

Generated diagrams sit between marker comments. A target replaces
everything between the markers and leaves the rest of the file alone. Never
edit inside the markers; edit the YAML.

    <!-- specarch:generate erDiagram -->
    ```mermaid
    erDiagram
      ...
    ```
    <!-- specarch:end -->

| Diagram | Marker | Source | Who draws it |
|---|---|---|---|
| `erDiagram` of entities, fields, keys, relations | `erDiagram` | `entities`, `relations` | generated |
| `stateDiagram-v2` of one entity with a `stateField` | `stateDiagram <Entity>` | `transitions` | generated |
| `sequenceDiagram` of one operation: client, service, algorithm, channels, answer | `sequenceDiagram <operationId>` | `paths`, `emits`, `algorithm` | generated |
| `sequenceDiagram` of one command: user, program, files read and written, exit statuses | `sequenceDiagram <command>` | `commands` | generated |
| `flowchart` of pages and their `actions` | `flowchart pages` | `pages` | generated |
| permissions matrix (a Markdown table, not a diagram) | `permissions` | `roles`, `permissions` | generated |
| context picture | | prose | hand-drawn |
| deployment picture | | prose | hand-drawn |
| any explanatory sketch of an algorithm or a flow | | prose | hand-drawn |

`specarch document techspec` fills every marked region of the `specarch.md`
beside a root file, and writes the whole technical specification, every
generated diagram included, to `techspec.md` in the folder it owns. A marker
for something the specification does not have is an error. The example in
`examples/library-lending/` shows both.

## The rule for new keywords

Before adding a keyword to the meta-model, look for one in JSON Schema,
OpenAPI, AsyncAPI or the standard of that stage that means the same thing
and take it with its exact meaning. Add a SpecArch keyword only when none
exists, mark it in the schema description and add a row to the origin table
above.
