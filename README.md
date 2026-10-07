# SpecArch

SpecArch is a specification language for whole systems. One set of files
describes the data model, the API, the events, the screens, the permissions,
the algorithms, the requirements they trace to and the decisions behind them.
People read the files, and so do AI assistants. Code, documents and diagrams
are generated from them.

SpecArch is personally owned and published under Apache-2.0. Anyone may use it
under that licence, including for commercial work. See `NOTICE` for the name
and for what you own in generated output.

## The format

A specification is a set of **SDF** files, SpecArch Definition Files. On disk
they are `*.specarch.yaml` for structure and `*.specarch.md` for explanation.
There is no bare `.sdf` extension: that one already means chemistry structure
files and SQL Server Compact databases.

The YAML uses the keywords of existing standards wherever one exists:

- JSON Schema for fields: `type`, `properties`, `required`, `enum`, `format`,
  `minimum`, `maxLength`, `pattern`.
- OpenAPI for endpoints: `paths`, `parameters`, `requestBody`, `responses`.
- AsyncAPI for events: `channels`, `messages`, `payload`.

SpecArch adds keywords only where no standard has one: relations between
entities, permissions on operations, pages, algorithms with worked examples,
requirement links and decision records. `docs/conventions.md` lists every
keyword with its origin.

Markdown carries the explanation and rationale. Mermaid carries the diagrams.
Diagrams that show structure (entity relations, state machines, request
sequences) are generated from the YAML, so they cannot drift from it; only
explanatory pictures are drawn by hand.

SpecArch has no grammar and no parser of its own. The language is defined by a
JSON Schema 2020-12 document, the meta-model, in `schema/`. The same schema
gives editors validation and completion through `yaml-language-server`: put
this on the first line of a file and most editors pick it up.

    # yaml-language-server: $schema=https://raw.githubusercontent.com/SpecArch/specarch/main/schema/specarch-0.1.schema.json

## The rules SpecArch serves

1. Spec first. The specification is written and reviewed before code. Code
   is generated from it. Only algorithm bodies are written by hand, and only
   after the algorithm is specified.
2. One specification covers data, API, events, UI, permissions,
   algorithms, requirements and decisions.
3. Spec and code stay in sync, enforced in CI. If code is ever written
   first, its spec is added in the same change and the merge waits until they
   match.
4. Algorithms are specified three ways: a formula, a worked numeric example
   and pseudocode. The worked examples become test cases.
5. Generated UI reuses the target project's component library, so it looks
   like the hand-built screens around it.
6. Existing code enters by extraction. An as-built spec is produced from the
   code, checked by regenerating and comparing, and from then on changes go
   spec first.

Two design choices follow from lessons learned on an earlier in-house language.
Access control is fail-closed: an operation with no permission is a validation
error, not a public endpoint. And nothing in the language is "written but not
enforced": a constraint, a formula or a worked example that no tool checks is
a defect in the roadmap, not an accepted state.

## Repository layout

| Path | Holds |
|---|---|
| `schema/specarch-0.1.schema.json` | the meta-model, JSON Schema 2020-12 |
| `docs/conventions.md` | YAML layout, Markdown sections, generated and hand-drawn diagrams |
| `docs/authoring-layer-evaluation.md` | TypeSpec, CUE and Pkl as an optional authoring layer |
| `docs/roadmap.md` | validator, generators, first real projects |
| `examples/library-lending/` | a small complete example that validates against the schema |

## Validating a specification

Until the SpecArch validator exists, any JSON Schema 2020-12 validator that
reads YAML will do. The repository uses `jv`, pinned by version and run with a
Go toolchain:

    go run github.com/santhosh-tekuri/jsonschema/cmd/jv@v0.7.0 -f \
      schema/specarch-0.1.schema.json examples/library-lending/library-lending.specarch.yaml

JSON Schema checks shape, types, required keys and identifier patterns. It
cannot check that a relation points at an entity that exists or that a page
lists only fields its entity has. Those cross-reference checks are the first
job of the validator CLI (see `docs/roadmap.md`), and until it ships a file
that passes the schema may still be inconsistent.

## Status

Version 0.1 of the meta-model, October 2026. The schema, the conventions and
one example exist. No validator CLI and no generator exist yet. The meta-model
will change: the first real projects written in SpecArch are expected to find
concepts it cannot express, and those gaps define v0.2.

## Licence

Apache License 2.0. See `LICENSE` and `NOTICE`.
