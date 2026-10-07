# SpecArch

SpecArch is a specification language for whole systems. One set of files
describes the data model, the API, the events, the screens, the permissions,
the algorithms, the requirements they trace to and the decisions behind them.
People read the files, and so do AI assistants. Code, documents and diagrams
are generated from them.

SpecArch is personally owned and published under Apache-2.0. Anyone may use it
under that licence, including for commercial work. See `NOTICE` for the name
and for what you own in generated output.

SpecArch is built on one principle, the Low IQ Tax: every file, key and
message should cost its reader as little thinking as possible. It is set out
in `docs/principles.md`, and the rest of SpecArch follows from it.

## The format

A specification has two kinds of file:

- **SDF**, SpecArch Definition File: the design, with no language,
  framework or build tool in it. On disk `*.specarch-design.yaml` for structure and
  `*.specarch-design.md` for explanation.
- **SIF**, SpecArch Implementation File: how one stack builds that design.
  On disk `<name>.<stack>.specarch-implementation.yaml`. It names the design file
  and version it implements and holds the language, toolchain, libraries
  with their licences, package layout, mappings, generator settings, build
  and test commands, deployments and implementation decisions. One design
  can have several.

The split follows one rule: what a client of an interface must know is
design; what only the builders or operators of one implementation need is
implementation. `docs/conventions.md` has the details, and both schemas
enforce them. There is no bare `.sdf` extension: that one already means
chemistry structure files and SQL Server Compact databases.

The YAML uses the keywords of existing standards wherever one exists:

- JSON Schema for fields: `type`, `properties`, `required`, `enum`, `format`,
  `minimum`, `maxLength`, `pattern`.
- OpenAPI for endpoints: `paths`, `parameters`, `requestBody`, `responses`.
  A standalone OpenAPI document is generated from the design file, not
  kept beside it.
- AsyncAPI for events: `channels`, `messages`, `payload`.

SpecArch adds keywords only where no standard has one: relations between
entities, permissions on operations, command-line commands, pages,
algorithms with worked examples, requirement links and decision records. `docs/conventions.md` lists every
keyword with its origin.

Markdown carries the explanation and rationale. Mermaid carries the diagrams.
Diagrams that show structure (entity relations, state machines, request
sequences) are generated from the YAML, so they cannot drift from it; only
explanatory pictures are drawn by hand.

SpecArch has no grammar and no parser of its own, apart from the small
expression language of checks and formulas. The language is defined by two
JSON Schema 2020-12 documents, the meta-model, in `schema/`: one for
design files and one for implementation files. The same schema
gives editors validation and completion through `yaml-language-server`: put
this on the first line of a file and most editors pick it up.

    # yaml-language-server: $schema=https://raw.githubusercontent.com/SpecArch/specarch/main/schema/specarch-design-0.1.schema.json

## The rules SpecArch serves

1. Spec first. The specification is written and reviewed before code. Code
   is generated from it. Only algorithm bodies are written by hand, and only
   after the algorithm is specified.
2. One specification covers data, API, events, UI, permissions,
   algorithms, requirements and decisions.
3. Spec and code stay in sync, enforced in CI (`docs/sync-gates.md`). If
   code is ever written first, its spec is added in the same change and the
   merge waits until they match.
4. Algorithms are specified three ways: a formula, a worked numeric example
   and pseudocode. The worked examples become test cases.
5. Generated UI reuses the target project's component library, so it looks
   like the hand-built screens around it.
6. Existing code enters by extraction (`docs/extraction.md`). An as-built
   spec is produced from the code, checked by regenerating and comparing, and
   from then on changes go spec first.

Two design choices follow from lessons learned on an earlier in-house language.
Access control is fail-closed: an operation with no permission is a validation
error, not a public endpoint. And nothing in the language is "written but not
enforced": a constraint, a formula or a worked example that no tool checks is
a defect in the roadmap, not an accepted state.

## Repository layout

| Path | Holds |
|---|---|
| `schema/specarch-design-0.1.schema.json` | the meta-model for design files, JSON Schema 2020-12 |
| `schema/specarch-implementation-0.1.schema.json` | the meta-model for implementation files |
| `spec/` | SpecArch's own specification: the design of the `specarch` command and its Go implementation |
| `docs/techspec/` | SpecArch's technical specification, generated from `spec/` |
| `conformance/` | the cases every implementation of `specarch` must pass |
| `history/` | what changed and why, one file per day |
| `docs/principles.md` | the Low IQ Tax principle and how SpecArch applies it |
| `docs/conventions.md` | YAML layout, Markdown sections, generated and hand-drawn diagrams |
| `docs/authoring-layer-evaluation.md` | TypeSpec, CUE and Pkl as an optional authoring layer |
| `docs/generators.md` | the rules every emitter follows and the pattern each target copies |
| `docs/sync-gates.md` | the CI checks that keep a spec and its code equal |
| `docs/extraction.md` | how an existing system gets its as-built spec, and what goes wrong |
| `docs/roadmap.md` | validator, generators, sync gates, first real projects, meta-model 0.2 |
| `examples/library-lending/` | a small complete example, with a design file and a Go implementation file |

## Installing and running the validator

`specarch` is one program, written in Go. With Go 1.26 or later:

    go install github.com/SpecArch/specarch/cmd/specarch@latest

or, from a clone of this repository:

    go install ./cmd/specarch

Check files, or folders of them:

    specarch validate spec/ examples/library-lending/

Every problem is one line: the file, the line, `error` or `warning`, the
YAML path, the rule, and one sentence saying what is wrong and how to fix
it.

    shop.specarch-design.yaml:58: error: /entities/Order/relations/customer/target: relation_target: Custmer is not an entity of this file; did you mean Customer?

The exit status is 0 when every file is valid (warnings may be printed), 1
when any file has an error, and 2 for a usage error, a path that cannot be
read, or a folder with no SpecArch file in it. `specarch version` prints the
program version and the meta-model versions it reads.

The validator applies the JSON Schema of each file's kind first, then what a
schema cannot express: references between objects, fail-closed access, the
boundary between design and implementation, expressions and their types,
worked examples evaluated to the last digit, concrete integer types, and the
test scenarios each subject needs. `docs/conventions.md` describes every
rule; the rule names are the `Rule` enum in `spec/specarch.specarch-design.yaml`.

The schemas also work alone, in an editor through `yaml-language-server`,
or with any JSON Schema 2020-12 validator that reads YAML.

## Generating the technical specification

    specarch generate techspec examples/library-lending/

writes `library-lending.techspec.md`, an arc42 technical specification with
Mermaid diagrams of the entities, states, operations, commands, pages and
permissions, into the folder the implementation file names (or `--out`). It
also refreshes the diagrams between `specarch:generate` markers in the
hand-written `library-lending.specarch-design.md`. It generates nothing from
a file with errors. With `--check` it writes nothing and exits 1 when the
committed output differs, which is how CI keeps the documents current.
`docs/techspec/specarch.techspec.md` is SpecArch's own, generated from
`spec/`.

## Status

Version 0.1 of the meta-model, October 2026. The two schemas, the
conventions, one example, SpecArch's own specification, the validator and
the techspec generator exist; the other generators are on the roadmap. The meta-model will change: the first real
projects written in SpecArch are expected to find concepts it cannot
express, and those gaps define v0.2.

## Licence

Apache License 2.0. See `LICENSE` and `NOTICE`.
