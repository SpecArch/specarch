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

### No change log inside a file

A design or implementation file says what is true now. Text such as
"previously", "changed from", "updated on" or "new in" does not belong in a
description; the history of a project lives in its own history files and in
git. Version fields stay, because they describe the current file. The
validator warns on the most common change-log phrases (`change_log`).

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
11. `tests`
12. `decisions`

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
| `tests`, `scenario`, `given`, `when`, `then`, `covers`, `notApplicable` | SpecArch | design tests; given, when and then are the Gherkin words, without Gherkin's file format |
| `testing`, `suites`, `designTests`, `designTestsOf`, `implementationOnly` | SpecArch | in implementation files: how one stack runs the design tests |

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
CEL, the Common Expression Language (cel.dev). Its syntax is the one C, Java
and JavaScript use, so most readers already know it, and its type rules are
strict: values of different types never mix without a written conversion.
CEL is strict on purpose. An implicit conversion is where precision is lost
without anyone seeing it (an int64 turned into a double, a decimal into a
float), so every conversion is written where it happens.

A check is one expression that gives a bool. A formula is one expression
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
| `size(x)` | `size(fullName) <= 200` | the length of a string or a list, an int |
| `round(x, places)` | `round(total, 2)` | a decimal or double rounded half away from zero; a decimal result has that scale |
| `floor(x)`, `ceil(x)` | `floor(hours)` | down or up to a whole number, keeping the type |
| `min(a, b, ...)`, `max(a, b, ...)` | `min(fee, replacementCost)` | the smallest or largest of two or more values of one type |

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
everything else. A design file's `tests` say what must hold on every stack;
an implementation file's `testing` says how one stack runs them, plus the
tests of its own code (unit and integration tests, fixtures, mocks,
performance targets, platforms).

A design test is about one subject: an `operation`, a `command`, a `page`,
or an `entity` with one of its `constraint`s or `transition`s. It is marked
`scenario: golden` for the path that succeeds or `scenario: red` for a path
that fails, and says what happens in three plain sentences: `given`, `when`
and `then`. There is no test language beyond that.

    tests:
      lend-limit-reached:
        operation: createLoan
        scenario: red
        covers: [response 409]
        given: a standard-tier member with three open loans
        when: createLoan is called for a fourth book
        then: it answers 409 and no loan is created

Every subject needs at least one golden scenario and its red ones. The
validator derives the cases a subject needs from the rest of the file, and
warns once for every case no test lists under `covers`, with a test to copy:

| Found in the design | Case | Scenario |
|---|---|---|
| a required field of a request body | `missing <field>` | red |
| `minimum` / `maximum` of a field or parameter | `<field> below minimum N` / `<field> above maximum N` | red |
| | `<field> at minimum N` / `<field> at maximum N` | golden |
| `exclusiveMinimum` / `exclusiveMaximum` | `<field> at exclusive minimum N` / `... maximum N` | red |
| `minLength` / `maxLength` | `<field> shorter than N characters` / `<field> longer than N characters` | red |
| | `<field> of N characters` | golden |
| `minItems` / `maxItems` | `<field> with fewer than N items` / `<field> with more than N items` | red |
| `pattern` | `<field> not matching its pattern` | red |
| an enum | `<field> not one of its values` | red |
| a format that values can break (email, uuid, date, decimal...) | `<field> not a valid <format>` | red |
| a permission other than `public` on an operation, command or page | `denied without <permission>` | red |
| a path parameter, or a route parameter of a page | `not found <parameter>` | red |
| a body field that is the `via` of a relation of the entity the operation returns | `not found <field>` | red |
| a unique constraint of the entity a POST with a 201 response creates | `duplicate <constraint>` | red |
| a channel the operation `emits` on | `dependency fails <channel>` | red |
| a 4xx or 5xx response | `response <status>` | red |
| a command | `usage error`, and `exit <status>` for each non-zero exit code | red |
| a check constraint / a unique constraint | `violates <constraint>` / `duplicate <constraint>` | red |
| a transition | `from wrong state` | red |

One test may cover several cases when they are one scenario (a lookup that
answers 404 covers `not found memberId` and `response 404`). A case that
does not apply is covered by a test with `notApplicable` and a written
reason instead of given, when and then. In 0.1 a missing scenario is a
warning, so a specification can be written before its tests; it is meant to
become an error in 0.2.

An implementation file's suite names the design tests it runs, by name
(`designTests`) or by subject (`designTestsOf: [{ command: validate }]`),
or says `implementationOnly: true`. The validator checks every name it
gives exists in the design file.

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

A section with nothing to say is left out; the others keep their numbers,
so section 8 is always cross-cutting concepts. A placeholder line would only
be noise for the reader.

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

`specarch generate techspec` fills every marked region of the
`<name>.specarch-design.md` beside a design file, and writes the whole
technical specification, every generated diagram included, to
`<name>.techspec.md` in the folder it owns. A marker for something the
design does not have is an error. The example in `examples/library-lending/`
shows both.

## The rule for new keywords

Before adding a keyword to the meta-model, look for one in JSON Schema,
OpenAPI or AsyncAPI that means the same thing and take it with its exact
meaning. Add a SpecArch keyword only when none exists, mark it in the schema
description and add a row to the origin table above.
