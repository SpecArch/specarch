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

Until its readers are built, extraction is a manual method with this
checklist, and each surface's reader is a small script kept in the
project's repository. `specarch extract <source>`, the verb that goes from
existing code or documents to a specification, is designed in
`spec/design/commands/extract.yaml` and built one reader at a time in the
steps of "Building extract" below. The Go build reads the sources
`outline` and `database`; a source not built yet is answered with status
2, and the Swift build has no extract verb.

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
- A code source is one repository, and its `edition` is the full hash
  of the commit the reader read, which the reader prints. It is the
  newest of the commits that last changed each path read, so a commit
  elsewhere in the repository leaves the output as it was. Git runs in
  the repository that holds the path, a submodule's own when the path is
  in one. Only tracked files are read; a path with changes not committed,
  or with files that are neither tracked nor ignored, is refused, since no
  commit names what was read, and so is a shallow clone, whose history
  cannot name the last change. A dump made from code, such as a
  catalogue or a route table, is itself committed and names the commit
  and the path it was made from; that commit is its edition, and a dump
  older than the last change to that path is refused as stale.
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

Merging is a verb of its own, `specarch merge --out <folder> <tree>...`,
since its input is specifications and not a surface: each tree is
extracted and checked on its own, then merged, and the merge is reviewed
apart from the readers. The same entity or operation from two trees
becomes one element with both citations. Where the sources disagree, or
only one of them has an element, the merge follows the table of
`docs/from-sources.md`, section 3.2, whose priority rule step 4 brings
to this: the question is `must` when the element concerns security (a role, a
permission, an operation, a limit, or a field that is personal or a
credential) or when the source that has it is one the system's owners
gave to parties outside, such as a published interface or a contract;
otherwise it is `should`. An undocumented route the code serves is the
usual way an open endpoint is found, so it is never left at `should`.

Steps, in order:

1. Built. The usage text marks `extract` as designed, not built, in both
   builds, and both builds refuse it with the same message.
2. Built. The shared core, the outline reader and the database reader,
   under the requirement SA-44, with `extract.yaml` holding the rules
   above. `extract outline` writes a source with its files as clauses and
   no elements, for a surface no reader reads yet, such as workflow
   definitions, so that `specarch gaps` lists it as not read rather than
   losing it. `extract database` reads a catalogue dump into entities:
   columns, types, nullability, defaults, primary and foreign keys,
   unique keys and checks; a check the expression subset can say becomes
   its expression, and a list of allowed values becomes the field's enum.
   An index, a view, a type with no row in the type-rendering idiom and a
   check the subset cannot say each print a line. SpecArch ships the
   catalogue query, `tools/catalogue/catalogue.sql`, and
   `tools/catalogue/dump-catalogue.sh`, which starts a disposable
   PostgreSQL under Podman, applies every migration and writes the dump
   with the commit it was made from; the dump is committed beside the
   code, so a run can be repeated without a database. The script is run
   by hand; CI reads the committed dump and needs no Podman. The lending
   desk's migration checks that the due date is fourteen days after the
   loan date, so the loan period is in the catalogue, and its dump is
   `examples/lending-desk/sources/catalogue/catalogue.json`. Conformance
   cases build a throwaway git repository with a fixed author, committer
   and dates, so the hashes they record never change.
3. The router reader, `extract router`: one operation per method and
   path pair, with its path parameters and the permission it checks, from
   a route table the running router prints, so that only routes actually
   registered appear. SpecArch defines the route table's format, as it
   ships the catalogue query. Done when the lending desk's four routes
   come out as four operations with their parameters and permissions,
   validated and byte-identical.
4. `specarch merge`, with the priority rule above, written into
   `docs/from-sources.md` section 3.2 as well, and an optional key on a
   source that marks it as given to parties outside, added to the 0.1
   design schema as the other keywords were. Done when the database
   and router trees of the lending desk merge into one specification
   that validates with no errors and `specarch gaps` reads, and a merge
   run twice is byte-identical. A release tag after this step lets the
   first project use the database and router readers.
5. A mark on an element that the project describes and compares but does
   not generate, naming the stakeholder that owns it: on the element's
   mapping in the implementation file, one element at a time, since one
   file often holds a single part another team owns; an optional key in
   the 0.1 implementation schema. Generators skip it; the comparisons and
   the gates still check it. Done when the lending desk marks one entity
   as owned by another stakeholder, `generate sql` and `generate openapi`
   leave it out while `validate` and `gaps` still read it, and a mark
   naming no stakeholder is a validation error in both builds.
6. The documents reader, `extract documents`, for Markdown. The source
   gets its outline as clauses: every heading, and the page or slide where
   the format has them. A sentence that makes a commitment (shall, must,
   will, a number, a time limit) becomes an element with `origin: stated`
   citing its clause; other prose produces no element and shows in the
   coverage. A commitment sentence the reader cannot place as one element
   becomes a question, never a guess. A field's sensitivity is read from a
   table column headed sensitivity, or from a sentence that calls the
   field personal or a credential; otherwise it is left for a question.
   Done when the lending desk manual
   gives its outline and its commitments, and merged with the code gives
   the deliberate disagreement on the loan period as one `must` question
   citing both. PDF and slide decks follow once a reader for them passes
   the dependency rules.
7. The OpenAPI reader, `extract openapi`, for OpenAPI 3.0 and 3.1:
   operations, parameters, request and response schemas. Where it and
   the router disagree, a question; a document none of whose paths the
   router serves is reported as a placeholder. Done when an OpenAPI file
   added to the lending desk, with one path the router does not serve,
   merges with the router tree into one question naming that path, and a
   file with only unserved paths is reported as a placeholder.
8. The permissions reader, `extract permissions`: roles and the
   permissions each grants, from the tables and seed scripts the running
   check reads. A check that runs only when a setting is present, and so
   passes everything when the setting is empty, is reported. The lending
   desk's roles move from a Go map to a roles table and a seed script.
   Done when its one role and three permissions come out validated and
   byte-identical, and a seed that grants a permission no route checks is
   reported.
9. The pages reader, `extract pages`, for a file-system router tree such
   as an `app/` folder: one page per folder that holds a page file, with
   dynamic segments as parameters and route groups recognised; and where
   a page has a schema file the component library renders from, the
   page's content from that schema. Done when an `app/` tree added to the
   lending desk, with a dynamic segment, a route group and one page with a
   schema file, gives one page per route with its parameters and that
   page's fields, validated and byte-identical.
10. The whole example: `examples/lending-desk/` extracted end to end by a
    script, with a personal column the manual has and the database does
    not, and a route the code serves without a permission that the
    manual does not mention, both listed by `specarch gaps` as `must`
    questions. CI runs the script twice and compares. A release tag after
    this step.

Not a step yet: a reader for workflow definitions. Meta-model 0.1 has no
object for a workflow binding (`flows` are a person's navigation across
pages); the workflow object is a 0.2 item, and until it exists the
outline reader records the workflow sources as not read.

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
