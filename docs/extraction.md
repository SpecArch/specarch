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
   `docs/refinement.md` is the design of that path, and
   `docs/from-sources.md` the procedure an agent follows when the system
   has both documents and code.

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
designed in `spec/design/commands/extract.yaml` and built in the steps of
"Building extract" below. Until a reader is built, every build answers it
with status 2.

## Building extract

The first real project to need `specarch extract` is a production service
written in Go behind an HTTP router, with its data in Postgres under
versioned migrations, a web interface on a file-system router, and several
hundred pages of design documents that have drifted from the code. The
verb is built for it in the steps below, each a work item of its own that
changes the specification of `specarch` first, adds its conformance cases,
and grows `examples/lending-desk/` with the sources it reads.

Every reader shares these rules:

- A reader reads one surface and writes a partial specification tree into
  `--out`: a root file with `tracksOrigin: true`, one source per document
  file and one per code repository, and every element `origin: stated`
  with `cites`, or `origin: inferred` with `why`. What the surface does not
  say is a question, never a value.
- A code source's `edition` is the full hash of the commit the reader
  read, and the reader prints it. It is the last commit that changed the
  path read, so a commit elsewhere in the repository leaves the output as
  it was; a path with changes not committed is refused, since no commit
  names what was read.
- Standard output is one line per object the meta-model could not hold,
  one line naming the commit read, and every count with what was counted
  and how.
- The same sources at the same commit give byte-identical output: no
  dates, no run times, no order taken from a map or the file system.
- A fact found in more than one place is reported at every place, with a
  question asking which one the running system uses, unless the reader
  can tell.
- A comment saying a file is generated from another source and must not
  be edited names that source; the reader reports it, so it is read and
  reconciled too.

Steps, in order:

1. Built. The usage text marks `extract` as designed, not built, in both
   builds, and the Swift build no longer says the Go build has it.
2. The shared core and the database reader. A requirement of its own for
   extraction, `extract.yaml` rewritten to the rules above, and
   `extract database`, which reads the catalogue of a database with every
   migration applied into entities: columns, types, nullability,
   defaults, primary, foreign and unique keys, indexes and checks. Done
   when `extract database` on the lending desk's catalogue writes a tree
   that `specarch validate` accepts with no errors, twice byte-identical,
   and a column type the meta-model cannot hold prints a line.
3. The router reader, `extract router`: one operation per method and
   path pair, with its path parameters, from a route table the running
   router lists. Done when the lending desk's four routes come out as
   four operations with their parameters, validated and byte-identical.
4. Merging, which joins the trees of several readers into one
   specification. The same entity or operation from two readers becomes
   one element with both citations; the rest follows the table of
   `docs/from-sources.md`, section 3.2. Done when the database and router
   trees of the lending desk merge into one specification that validates
   with no errors and `specarch gaps` reads, and a merge run twice is
   byte-identical. A release tag after this step lets the first project
   use the database and router readers.
5. The documents reader, `extract documents`, for Markdown: the source
   with its outline as clauses, and what the text states in a form a
   program can read without guessing. Done when the lending desk manual
   gives its outline and its stated elements, each cited to its heading,
   and merged with the code gives the deliberate disagreement on the loan
   period as one `must` question citing both. PDF and slide decks follow
   once a reader for them passes the dependency rules.
6. The OpenAPI reader, `extract openapi`, for OpenAPI 3.0 and 3.1:
   operations, parameters, request and response schemas. Where it and
   the router disagree, a question; a document none of whose paths the
   router serves is reported as a placeholder.
7. The permissions reader, `extract permissions`: roles and the
   permissions each grants, from the tables and seed scripts the running
   check reads. A check that runs only when a setting is present, and so
   passes everything when the setting is empty, is reported.
8. The pages reader, `extract pages`, for a file-system router tree: one
   page per route, with dynamic segments as parameters and route groups
   recognised.
9. The whole example: `examples/lending-desk/` extracted end to end by a
   script, with a column the manual has and the database does not, and a
   route the code serves and the manual does not mention, both listed by
   `specarch gaps`. CI runs the script twice and compares. A release tag
   after this step.

Waiting on the owner, and not yet a step: a reader for workflow
definitions, since meta-model 0.1 has no object for a workflow binding
(`flows` are a person's navigation across pages), and a way to mark an
element that the project describes and compares but does not generate,
for the parts another team owns.

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
