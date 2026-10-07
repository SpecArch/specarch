# Conventions

How a SpecArch specification is laid out on disk, how the YAML is organised,
what the Markdown sections are, and which diagrams are generated.

## Files

A project keeps its specification in a `spec/` folder at the repository root
(or wherever its own rules say). Two kinds of file live there:

| File | Holds |
|---|---|
| `<name>.specarch-design.yaml` | the design (SDF): everything a generator reads about what the system is and does |
| `<name>.specarch-design.md` | the explanation: everything written for a person |
| `<name>.<stack>.specarch-implementation.yaml` | an implementation (SIF): how one stack builds the design |
| `<name>.<stack>.specarch-implementation.md` | optional explanation of that implementation |

A small system fits in one YAML file. A larger one will split by bounded
context, one pair of files per context, each a complete document with its own
`info`. Meta-model 0.1 has no cross-file references: a `$ref` and a relation
`target` can only name an entity or enum in the same file, and the schema
rejects anything else. References between contexts are a v0.2 item in
`docs/roadmap.md`.

The first line of every YAML file is the editor hint. For a design file:

    # yaml-language-server: $schema=https://raw.githubusercontent.com/SpecArch/specarch/main/schema/specarch-design-0.1.schema.json

For an implementation file:

    # yaml-language-server: $schema=https://raw.githubusercontent.com/SpecArch/specarch/main/schema/specarch-implementation-0.1.schema.json

Pin the schema version a project is written against. A project moves to a new
meta-model version on purpose, in its own change.

## Design and implementation

SpecArch has two kinds of file, and every fact belongs in exactly one of
them.

A **SpecArch Definition File** (SDF, `*.specarch-design.yaml`, root key
`specarch`) is the design. It is neutral about language, compiler and
framework: entities, enums, operations, commands, events, pages,
permissions and roles, algorithms with formula, worked examples and
pseudocode, requirement links, and the decisions that hold whatever the
stack.

A **SpecArch Implementation File** (SIF, `*.specarch-implementation.yaml`, root key
`specarchImplementation`) is one stack's implementation of one design file. It
names that file and the `info.version` it was written against under
`implements`, and holds everything that depends on the stack and nothing
else: the language and toolchain with versions (`target`), every library
with its version and licence (`libraries`), the package layout and the
design objects each package implements (`layout`), how each design object
maps onto the stack (`mappings`), the framework per interface kind
(`bindings`), generator targets and their settings (`generators`), build,
test and CI commands (`tasks`), real servers, hosts and ports
(`deployments`), and implementation decisions (`decisions`). Its schema,
`schema/specarch-implementation-0.1.schema.json`, has no keyword for an entity, an
operation or any other design object, so an implementation file cannot add
or change design. One design file can have several implementation
files, one per stack; the `<stack>` part of the name tells them apart
(`library-lending.go.specarch-implementation.yaml`).

Pointers from an implementation file into its design file are JSON pointers
written from the design file's root: `#/entities/Loan`,
`#/commands/validate`, `#/paths/~1loans/post` (a `/` inside a key is
written `~1`). The validator checks that `implements` names a design
file with the same `info.version` and that every pointer resolves, so a
design change is noticed by every implementation of it.

Generators read the design file for the design and the implementation for
the target choices: which folder a target owns, the settings of the code
generator that follows it, the type a decimal maps to.

### The interface boundary

The rule of thumb: if a client of the interface needs to know it, it is
design and goes in the design file. If only the people building or
running one particular implementation need it, it goes in the
implementation file.

| Design file (design) | Implementation file (one stack) |
|---|---|
| the kind of interface: HTTP (`paths`), messaging (`channels`), command line (`commands`) | the framework binding: router, middleware, how handlers implement the generated interface; the CLI library |
| operations with operationId, method and path; parameters, request bodies, responses with schemas, status codes and error shapes | code-generator configuration: package names, strict-server mode, `x-oapi-codegen-*` extensions, protoc plugins |
| channels and messages | the messaging client and its settings |
| commands, arguments, options and exit codes | |
| the permission each operation, command and page needs; roles | servers, hosts, ports and environments of real deployments |

A standalone `openapi.yaml` or `asyncapi.yaml` is generated from the
design file. It is never a third source kept by hand.

Both schemas enforce the boundary. The design schema refuses extension
keys that name one implementation: `x-oapi-codegen-*`, `x-go-*` and the
other language prefixes, `x-server`, `x-host`, `x-port`, `x-deploy`,
`x-router`, `x-framework` and the rest of the `stackSpecificKey` pattern in
the schema. The implementation schema is closed and has no design keyword.
The validator reports each case with its own rule, `stack_key` or
`design_key`. Other interface kinds (gRPC, file exchange, a serial protocol)
have no keyword yet; they are 0.2 items in `docs/roadmap.md`.

## YAML layout

Top-level keys appear in this order. A generator does not care; a reader does.

1. `specarch`, `info`
2. `requirementSources`
3. `enums`
4. `entities`
5. `permissions`, `roles`
6. `paths`
7. `commands`
8. `channels`
9. `pages`
10. `algorithms`
11. `decisions`

Inside an entity: `description`, `type`, `properties`, `required`,
`primaryKey`, `relations`, `constraints`, `stateField`, `transitions`,
`requirements`. Fields appear in the order they would be shown on a form:
identity first, then the fields a person fills in, then the fields the system
sets.

Short objects are written on one line with flow style:
`{ type: string, format: uuid, readOnly: true }`. Anything with a
`description` longer than a few words goes block style. Prose longer than one
line uses a literal block (`|`).

Three values are always quoted, because YAML would otherwise read them as
something else: dates (`date: "2026-10-07"`, which YAML takes for a
timestamp), decimal amounts (`"0.50"`, which YAML takes for a float), and any
flow-style string containing a comma, a colon or a question mark. The schema
rejects the unquoted forms, so the mistake is caught, but it is better not to
make it.

### Names

| Thing | Case | Example |
|---|---|---|
| entity, enum, message | PascalCase | `Loan`, `LoanStatus`, `LoanCreated` |
| field, relation, algorithm, operationId, input | camelCase | `dueOn`, `createLoan` |
| enum value | snake_case | `set_null`, `overdue` |
| constraint | snake_case, prefixed with the entity | `loan_due_after_loaned` |
| permission | dotted lower-case, `area.verb` | `loans.create` |
| role, page | kebab-case | `librarian`, `members-list` |
| channel | dotted lower-case | `loan.lifecycle` |
| command | kebab-case words separated by spaces | `validate`, `generate techspec` |
| command argument, option | kebab-case | `paths`, `out` |
| decision | `ADR-` and three or more digits | `ADR-001` |
| requirement link | `SOURCE-id` | `LIB-5` |

The schema enforces these patterns. Generators rely on them to derive names in
the target (table names, URL segments, constant names) without a mapping table.

### Where each keyword comes from

Every keyword in the meta-model is either borrowed from a standard, with its
standard meaning, or marked as SpecArch's own. Borrowed keywords are never
redefined.

| Keyword | Origin | Notes |
|---|---|---|
| `info`, `paths`, `parameters`, `requestBody`, `responses`, `content`, `operationId`, `summary`, `deprecated` | OpenAPI 3 | bounded subset: `in` is path, query or header; no callbacks, links or servers |
| `type`, `properties`, `required`, `enum`, `const`, `format`, `default`, `examples`, `minimum`, `maximum`, `exclusiveMinimum`, `exclusiveMaximum`, `multipleOf`, `minLength`, `maxLength`, `pattern`, `items`, `minItems`, `maxItems`, `uniqueItems`, `readOnly`, `writeOnly`, `$ref` | JSON Schema 2020-12 | `$ref` is restricted to `#/entities/X` and `#/enums/X`; nullability is written `type: [string, "null"]` as JSON Schema does |
| `channels`, `messages`, `payload` | AsyncAPI | one level: channel, messages, payload; no servers, bindings or operations objects |
| `x-*` | OpenAPI convention | allowed in every object, ignored by validation |
| `requirementSources`, `requirements` | SpecArch | requirement links into an external set (ISO/IEC/IEEE 29148 traceability); see "Requirement links" below |
| `primaryKey`, `relations`, `constraints`, `stateField`, `transitions` | SpecArch | data-model concepts JSON Schema has no words for |
| `precision`, `scale` | SpecArch | decimal size; JSON Schema has no decimal type, so `format: decimal` on a string carries them |
| `valueDescriptions` | SpecArch | per-value meaning of an enum |
| `commands`, `arguments`, `options`, `reads`, `writes`, `standardOutput`, `standardError`, `exitCodes`, `repeatable` | SpecArch | command-line interfaces; no standard describes one |
| `permissions`, `roles`, `permission` | SpecArch | OpenAPI's `security` names a scheme, not a right; SpecArch needs the right |
| `emits`, `algorithm` | SpecArch | links from an operation to its events and its computation |
| `pages`, `kind`, `route`, `entity`, `source`, `submit`, `columns`, `fields`, `filters`, `actions` | SpecArch | UI page definitions |
| `algorithms`, `inputs`, `output`, `formula`, `examples` (of an algorithm), `pseudocode` | SpecArch | IEEE 1016 algorithm viewpoint, made testable |
| `decisions` and the ADR fields | SpecArch | the common ADR shape: context, decision, consequences |

### Requirement links

Every named object carries a `requirements` list: entity, enum, relation,
constraint, transition, permission, role, operation, command, channel,
message, page, algorithm and decision. A field, a parameter, a response, an action or a
worked example does not; each traces through the object that holds it. The
list is optional, because not every object serves a requirement a project
has written down, but a generator that builds a traceability matrix treats
an object with no links as a gap to show, not as fully covered.

### Access control is fail-closed

Every operation, command and page names exactly one `permission`. Leaving it out
is a schema error. The permission `public` is reserved for things open to
everyone and must be written out; there is no default. A role lists the
permissions it grants and cannot be empty. Row-level rules (a member sees only
their own loans) are not in the meta-model yet; they are described in prose and
enforced by the service, and are on the list for v0.2.

### Expressions

`check` constraints and `formula` strings are written in a small subset of
CEL, the Common Expression Language (cel.dev). Its syntax is the one C,
Java and JavaScript use, so most readers already know it. A check is one
expression that gives true or false. A formula is one expression that gives
the algorithm's output; it has no `name =` in front. The validator parses
every expression, refuses anything outside the subset by name, checks the
names and the types, and evaluates every formula on its worked examples.

| Part | Example | Meaning |
|---|---|---|
| number | `21`, `0.50` | an exact number; there is no binary floating point |
| text | `"open"`, `'open'` | a string, in double or single quotes |
| `true`, `false` | `active == true` | the two booleans |
| `null` | `returnedAt == null` | no value; only for a field that allows null |
| name | `dueOn` | a field of the entity (in a check) or an input of the algorithm (in a formula) |
| `( )` | `(a + b) * c` | grouping |
| `-x` | `-discount` | negation |
| `!x` | `!active` | not |
| `*` | `daysLate * dailyRate` | multiplication |
| `/` | `total / count` | division, exact |
| `+` | `subtotal + tax` | addition |
| `-` | `price - discount` | subtraction of numbers |
| `==`, `!=` | `status == "open"` | equal, not equal |
| `<`, `<=`, `>`, `>=` | `copiesAvailable <= copiesOwned` | order of numbers, dates and date-times |
| `&&` | `a > 0 && b > 0` | and |
| `\|\|` | `returnedAt == null \|\| returnedAt >= loanedAt` | or |
| `c ? a : b` | `daysLate > 0 ? daysLate * dailyRate : 0` | `a` when `c` holds, otherwise `b` |
| `size(x)` | `size(fullName) <= 200` | length of a text (CEL's own function) |
| `date(x)` | `dueOn > date(loanedAt)` | the date of a date-time, or a quoted date such as `date("2026-10-07")` |
| `min(a, b, ...)` | `min(daysLate * dailyRate, replacementCost)` | the smallest of two or more numbers, dates or date-times |
| `max(a, b, ...)` | `max(balance, 0)` | the largest of two or more |
| `round(x, places)` | `round(total, 2)` | rounds half away from zero to that many decimal places |

Nothing else is in the subset: no `%`, no `in`, no lists or maps, no field
access with `.`, no macros (`has`, `all`, `exists`, `map`, `filter`), and no
other function. Each of these is refused with a message naming it. A
comparison does not chain: `a < b < c` is a type error, written
`a < b && b < c`. Anything a real specification needs beyond this list is
added on purpose, in its own change, with an example here.

Types come from the fields. `integer`, `number` and a string with
`format: decimal` are numbers; `format: date` and `format: date-time` are
dates and date-times, which do not mix (use `date(x)`); a `$ref` to an enum
is compared with a string of one of its values; a field that allows null may
be compared with `null`, and one that does not may not. `&&`, `||`, `!`, the
condition of `? :` and a check's result need booleans.

A worked example's inputs and expected value are typed by the algorithm's
`inputs` and `output`: an integer is a YAML integer, a decimal a quoted
string (`"0.50"`), a date a quoted date. When the output is a decimal with a
`scale`, the result is rounded to that scale before it is compared, so
`"3.50"` and `3.5` are equal and a third with scale 2 is `"0.33"`.

### What the schema cannot check

JSON Schema validates shape: types, required keys, patterns, closed objects.
It does not know that `target: Loan` must name an entity in the same file,
that a `primaryKey` field must exist in `properties`, that a page's `columns`
belong to its `entity`, that a `transition` uses values of the state field's
enum, or that a `requirements` entry has a prefix in `requirementSources`.
Those are cross-reference checks, made by `specarch validate`
together with the expression, worked-example and access checks. A file that
passes the schema can still fail the validator.

## Markdown sections

The `.specarch-design.md` file beside the YAML explains it. The sections follow
arc42, trimmed to what a small system needs. Each heading is fixed so a
generator can find it and so readers of different specifications know where
to look.

1. Introduction and goals: what the system is for, who uses it, the top
   quality goals.
2. Constraints: technical, organisational, legal.
3. Context: the system and its neighbours. One hand-drawn context picture.
4. Solution strategy: the few decisions that shape everything else, each
   pointing at its ADR.
5. Building blocks: generated entity diagram, then prose on each entity
   group.
6. Runtime view: generated sequence diagrams for the main operations,
   generated state diagram for each entity with a `stateField`.
7. Deployment: hand-drawn.
8. Cross-cutting concepts: permissions table (generated), money, time,
   identifiers, error handling.
9. Architecture decisions: generated list from `decisions`, with a link to
   each.
10. Quality requirements: scenarios.
11. Risks and technical debt.
12. Glossary.
13. Requirements: the requirement list when the project keeps it in the
    spec rather than in an external tool. Each entry is the ID the YAML's
    `requirements` links point at.

Not every section needs text. An empty section stays in the file with the
line "Nothing yet." so its absence is visible rather than silent.

## Mermaid diagrams: generated or hand-drawn

Generated diagrams sit between marker comments. A generator replaces
everything between the markers and leaves the rest of the file alone. Never
edit inside the markers; edit the YAML.

    <!-- specarch:generate erDiagram -->
    ```mermaid
    erDiagram
      ...
    ```
    <!-- specarch:end -->

| Diagram | Source | Who draws it |
|---|---|---|
| `erDiagram` of entities, fields, relations | `entities`, `relations` | generated |
| `stateDiagram-v2` per entity with a `stateField` | `transitions` | generated |
| `sequenceDiagram` per operation: client, service, store, channel | `paths`, `emits`, `algorithm` | generated |
| `flowchart` of pages and their `actions` | `pages` | generated |
| permissions matrix (a Markdown table, not a diagram) | `roles`, `permissions` | generated |
| context picture | prose | hand-drawn |
| deployment picture | prose | hand-drawn |
| any explanatory sketch of an algorithm or a flow | prose | hand-drawn |

Generators do not exist yet. Until they do, generated diagrams are written by
hand inside the markers and checked against the YAML in review. The example in
`examples/library-lending/` shows the markers in use.

## The rule for new keywords

Before adding a keyword to the meta-model, look for one in JSON Schema,
OpenAPI or AsyncAPI that means the same thing and take it with its exact
meaning. Add a SpecArch keyword only when none exists, mark it in the schema
description and add a row to the origin table above.
