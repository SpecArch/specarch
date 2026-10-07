# Extraction

How an existing system gets its as-built specification, and what goes wrong
when that is done carelessly. This is the method behind the sixth rule in
the README: existing code enters by extraction, and from then on changes go
spec first.

## The method

1. Find the authoritative source for each surface. The place the running
   system actually reads a fact from, not the place the documentation says it
   lives. The table below lists the usual ones.
2. Extract mechanically. Each surface is read by a script or a query into the
   as-built spec. Nothing is typed in from a document or from memory.
3. Verify by regenerating. Run the generators on the as-built spec and
   compare their output with the source: routes against the router, tables
   against the catalogue, generated code against the committed code. Record
   every difference.
4. Only then review by hand. A reviewer reads the spec, fixes what the
   extraction got wrong, and marks it the master. From that point a
   difference between spec and code is a defect in the code, and the gates in
   `docs/sync-gates.md` keep it that way.
5. Record what the meta-model could not hold. Every concept the code has that
   the spec cannot express is listed with the extraction. Those lists are the
   input to the next meta-model version.
6. Write what the source does not say as open questions, never as guesses.
   Every element carries its `origin`: stated, with the citation of where
   the source says it; inferred, with the reason; or decided, once a
   stakeholder has answered. `specarch gaps` lists what is still open and
   which outputs can already be made; code is generated only once the
   questions are answered and the owner has approved the documents.
   `docs/refinement.md` is the design of that path.

| Surface | Authoritative source | How to read it |
|---|---|---|
| endpoints | the built router | list its registered method and path pairs |
| data model | the database catalogue after every migration is applied | dump tables, columns, keys, indexes, checks |
| permissions | wherever the running authorisation check reads its rules | often tables filled by seed scripts, not a policy file |
| events | the publisher calls and the topic or channel registry | list topics, then the payload type each publisher sends |
| pages | the schema objects the component library renders from | read them as data |
| algorithms | the code, with the numbers from an existing test or a real record | the record becomes the first worked example |
| decisions | commit history, review threads, chat | the only surface that is written by hand |

Extraction is a manual method with this checklist for the first projects.
Each surface's reader is a small script kept in that project's repository.
The readers that prove general become `specarch extract <source>`, the verb
that goes from existing code or documents to a specification; it is
designed in `spec/design/commands/extract.yaml` and built when the first
real project needs it. Until then every build answers it with status 2.

## What goes wrong

Each of these was met in practice.

### The obvious source is wrong

In one system the authorisation library's adapter was a stub that granted
nothing, and the real permissions lived in database tables that seed scripts
filled. Extracting from the library's policy file would have produced a
confident, wrong model with every check passing against it. Trace the running
check to where it reads before extracting anything.

### Prose specifications lie quietly

A data-model document listed tables that no migration had ever created, and
nobody had noticed because nothing compared the two. Documentation is a list
of questions to put to the running system, never a source to copy from.

### Template files mislead

A service template ships with a sample OpenAPI file, and most services built
from the template still carry it unchanged: a handful of example paths, none
of them served, next to the real routes the router serves. A tester handed
that file tests endpoints that do not exist and misses the ones that do. A
placeholder spec is worse than none, so the extractor flags one: an OpenAPI
file none of whose paths the router serves is reported as a placeholder, and
the route-table gate refuses it.

### Moving every handler at once

Putting every working handler of a service onto a generated server interface
in one change puts a rewrite in front of the next release, and the rewrite
loses. New endpoints go spec first from the day the gate is on. Existing
handlers move service by service, each when it is next touched, behind the
route-table gate and its shrinking allowlist.

### Typing data changes into a live system

A new role and its permissions, a backfill, a correction: each was run by
hand against production at some point. They are safer emitted from the spec
as a guarded script, with preconditions that raise, a dry run by default,
postconditions that check exact counts and no effect on a second run. That
emitter waits on the `guard` concept in meta-model 0.2.

## What extraction asked of the meta-model

Extracting real systems asks for four concepts. Meta-model 0.1 has one of
them; the other three are 0.2 items, because a keyword that no tool checks is
a defect, and only the validator or an emitter can check them.

- Trace, in 0.1: every named object carries `satisfies`, the requirements
  it meets, and every test `verifies`. Fields, parameters, responses,
  actions and worked examples trace through the object that holds them.
- Guard, 0.2: preconditions and postconditions on a data change, with exact
  expected counts. A new top-level object; its emitter is the guarded script
  above.
- Separation of duties, 0.2: sets of permissions that one holder must never
  have together. The schema holds the shape; the validator checks that no
  role grants two of them.
- Flow, 0.2: a workflow binding (trigger, form schema, steps) as an object
  of its own, not an annotation on an endpoint.
