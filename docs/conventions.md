# Conventions

How a SpecArch specification is laid out on disk, how the YAML is organised,
what the Markdown sections are, and which diagrams are generated.

## Files

A project keeps its specification in a `spec/` folder at the repository root
(or wherever its own rules say). Two kinds of file live there:

| File | Holds |
|---|---|
| `<name>.specarch.yaml` | the structure: everything a generator reads |
| `<name>.specarch.md` | the explanation: everything written for a person |

A small system fits in one YAML file. A larger one will split by bounded
context, one pair of files per context, each a complete document with its own
`info`. Meta-model 0.1 has no cross-file references: a `$ref` and a relation
`target` can only name an entity or enum in the same file, and the schema
rejects anything else. References between contexts are a v0.2 item in
`docs/roadmap.md`.

The first line of every YAML file is the editor hint:

    # yaml-language-server: $schema=https://raw.githubusercontent.com/SpecArch/specarch/main/schema/specarch-0.1.schema.json

Pin the schema version a project is written against. A project moves to a new
meta-model version on purpose, in its own change.

## YAML layout

Top-level keys appear in this order. A generator does not care; a reader does.

1. `specarch`, `info`
2. `requirementSources`
3. `enums`
4. `entities`
5. `permissions`, `roles`
6. `paths`
7. `channels`
8. `pages`
9. `algorithms`
10. `decisions`

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
| `requirementSources`, `requirements` | SpecArch | requirement links into an external set (ISO/IEC/IEEE 29148 traceability) |
| `primaryKey`, `relations`, `constraints`, `stateField`, `transitions` | SpecArch | data-model concepts JSON Schema has no words for |
| `precision`, `scale` | SpecArch | decimal size; JSON Schema has no decimal type, so `format: decimal` on a string carries them |
| `valueDescriptions` | SpecArch | per-value meaning of an enum |
| `permissions`, `roles`, `permission` | SpecArch | OpenAPI's `security` names a scheme, not a right; SpecArch needs the right |
| `emits`, `algorithm` | SpecArch | links from an operation to its events and its computation |
| `pages`, `kind`, `route`, `entity`, `source`, `submit`, `columns`, `fields`, `filters`, `actions` | SpecArch | UI page definitions |
| `algorithms`, `inputs`, `output`, `formula`, `examples` (of an algorithm), `pseudocode` | SpecArch | IEEE 1016 algorithm viewpoint, made testable |
| `decisions` and the ADR fields | SpecArch | the common ADR shape: context, decision, consequences |

### Access control is fail-closed

Every operation and every page names exactly one `permission`. Leaving it out
is a schema error. The permission `public` is reserved for things open to
everyone and must be written out; there is no default. A role lists the
permissions it grants and cannot be empty. Row-level rules (a member sees only
their own loans) are not in the meta-model yet; they are described in prose and
enforced by the service, and are on the list for v0.2.

### Expressions

`check` constraints and `formula` strings are plain text in 0.1. The
expression language is deliberately small and will be fixed when the validator
CLI starts checking it: field names, literals, `null`, the comparison and
arithmetic operators, `and`, `or`, `not`, and the functions `date()`,
`min()`, `max()`, `len()`. Until then a generator copies the expression into
the target with the operators translated, and a reviewer checks it by eye.

### What the schema cannot check

JSON Schema validates shape: types, required keys, patterns, closed objects.
It does not know that `target: Loan` must name an entity in the same file,
that a `primaryKey` field must exist in `properties`, that a page's `columns`
belong to its `entity`, that a `transition` uses values of the state field's
enum, or that a `requirements` entry has a prefix in `requirementSources`.
Those are cross-reference checks and they are the first job of the validator
CLI. A file that passes the schema today can still be inconsistent.

## Markdown sections

The `.specarch.md` file beside the YAML explains it. The sections follow
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
