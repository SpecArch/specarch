# Authoring layer: TypeSpec, CUE and Pkl

SpecArch files are YAML validated by a JSON Schema. The question here is
whether a richer language should sit on top, so that a person or an AI writes
the specification in that language and compiles it to the YAML. Three
candidates were looked at: TypeSpec (Microsoft), CUE and Pkl (Apple). Each was
judged against four tests. The release and activity figures were read from
each project's GitHub repository on 2026-10-07.

## The four tests

1. Can an AI write it without errors? An assistant producing a spec from a
   conversation must get the syntax right first time, most of the time. The
   measure is how much of the language is plain data versus constructs with
   their own rules, and how much published material exists for the model to
   have learned from.
2. Can the owner read it? The specification is reviewed before code. A
   reviewer must be able to read a file cold and see what it says, without
   running a tool.
3. Can it emit the needed targets? SpecArch needs, at minimum, the YAML
   shape of its own meta-model; on its own terms the language should also
   produce OpenAPI, JSON Schema and AsyncAPI, since those are the standards the
   YAML borrows from.
4. Is it maintained? Releases in the last year, an active repository, a
   named maintaining organisation.

## TypeSpec

A TypeScript-flavoured language for describing APIs: models, operations,
interfaces, decorators. Compiles through emitters.

- *Maintenance*: active. Stable release 1.16.0 on 2026-09-09, a compiler
  commit on 2026-10-07, maintained by Microsoft and used for Azure's own API
  definitions. Passes.
- *Targets*: emitters for OpenAPI 3, JSON Schema and Protobuf ship with the
  project; no AsyncAPI emitter does. A SpecArch emitter would have to be
  written, in TypeScript, and it would have to invent decorators for
  everything OpenAPI does not cover: pages, algorithms, permissions, decisions,
  relations. Those decorators have no meaning outside our emitter. Partial pass.
- *Readability*: good for models and operations, which look like TypeScript
  interfaces. A reviewer who does not program in TypeScript reads it slower
  than YAML. Decorators stacked on a model (`@route`, `@doc`, `@key`,
  `@visibility`) read as noise until learned. Pass with a cost.
- *AI authoring*: strong for the API part, since TypeSpec is well represented
  in training material and resembles TypeScript. Weak for everything SpecArch
  adds: an assistant would be writing decorators that exist nowhere else, with
  no examples to learn from. Partial pass.

## CUE

A data language: JSON with types, constraints and unification. A CUE file can
validate data, define schemas and generate JSON or YAML. It exports OpenAPI 3
and imports JSON Schema and OpenAPI.

- *Maintenance*: active. Stable v0.17.1 on 2026-07-16, v0.18.0 alphas through
  2026-10-05, commits on 2026-10-06. Still below 1.0 after many years, and the
  release notes show language features arriving as experiments behind flags.
  Passes, with the caveat that the language is still moving.
- *Targets*: `cue export` writes YAML or JSON of any shape, so the SpecArch
  YAML is a direct export with no emitter to write. OpenAPI export exists for
  definitions. No AsyncAPI. The meta-model itself could be expressed in CUE and
  exported as JSON Schema, keeping one source. Strong pass.
- *Readability*: CUE that is mostly data reads like JSON without the quotes
  and is clear. CUE that uses its power (unification, definitions, disjunctions,
  comprehensions, hidden fields) is hard to read cold, and the question "what
  does this evaluate to" needs the tool. Pass for plain use, fail for idiomatic
  use.
- *AI authoring*: the weakest point. CUE's rules on when two values unify, on
  open versus closed structs and on definitions versus regular fields are
  subtle, error messages are terse, and there is far less published CUE than
  TypeScript or YAML. In practice an assistant writes CUE that fails to
  evaluate, then iterates with the tool. Fail for authoring from scratch;
  usable for validation written once by a person.

## Pkl

Apple's configuration language: classes, typed properties, amends, modules.
Renders to JSON, YAML, XML, property lists. Code generators for Java, Kotlin,
Swift and Go produce typed bindings for the configuration.

- *Maintenance*: active. 0.32.1 on 2026-07-23, commits on 2026-10-06,
  maintained by Apple. Passes.
- *Targets*: YAML and JSON of any shape, so the SpecArch YAML is a direct
  render. No OpenAPI, JSON Schema or AsyncAPI output. The Swift and Go
  generators are interesting for the owner's own stacks, but they generate
  bindings for reading configuration, not application code. Partial pass.
- *Readability*: good. A Pkl module with classes and typed properties reads
  like a cleaner YAML with types. `amends` and late binding take a moment to
  learn, after which files are easy to follow. Pass.
- *AI authoring*: moderate. The syntax is regular and small, but Pkl is young
  and little of it exists in training material, so an assistant makes
  mistakes on the module system, on `amends` versus `extends` and on the
  standard library. Better than CUE, worse than TypeSpec. Partial pass.

## Summary

| Test | TypeSpec | CUE | Pkl |
|---|---|---|---|
| AI writes it without errors | partial | fail | partial |
| Owner reads it cold | pass, slower | pass only for plain data | pass |
| Emits the needed targets | OpenAPI, JSON Schema; no AsyncAPI; SpecArch emitter to write | any YAML; OpenAPI; no AsyncAPI | any YAML; no standards output |
| Maintained | pass | pass, still pre-1.0 | pass |

## Recommendation

Do not add an authoring layer for 0.1. Write SpecArch files directly as YAML.

The reason is the first test. The YAML already is the format the three
languages would compile to, and YAML with an editor schema is the format an
assistant writes with the fewest errors and a person reviews with the least
effort. Every candidate adds a compile step and a second syntax to learn, and
each one loses on at least one test that YAML passes outright: TypeSpec has no
words for half the meta-model, CUE is hard for an assistant to write and for a
reviewer to evaluate in their head, and Pkl emits none of the standards
SpecArch builds on.

Two of them stay useful in supporting roles:

- CUE as a second validator. The cross-reference rules the JSON Schema
  cannot express can be written in CUE and run against the exported YAML in
  CI, as an independent check of the validator. Written once by a person, this
  plays to CUE's strength and avoids its weakness.
- TypeSpec for teams that already have it. A project whose API is already
  in TypeSpec can keep it there and emit OpenAPI, and SpecArch can import that
  OpenAPI into the `paths` section. That is an import path, not an authoring
  layer, and belongs to the extraction tooling in the roadmap.

Revisit after the first real projects (roadmap step 4). If writing the YAML by
hand proves too repetitive at that scale, Pkl is the candidate to try first:
it reads well and renders YAML directly, so a Pkl module could define the
repeated shapes once. The decision will rest on the first test again, measured
on real files rather than argued.
