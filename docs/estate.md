# Specifying an estate of modules

An estate of some twenty modules, each in a repository of its own and
owned by a different team: Go services behind an HTTP router, with
PostgreSQL migrations and an authorisation policy of path patterns;
admin applications on a file-system router; workflow definitions in JSON
that are not BPMN; and Markdown documents mostly in a language other
than English. The team specifying it works from an umbrella repository
that holds the code repositories as git submodules and may not commit to
any of them. It wants one specification per module, valid with no
errors, approved, and then documents and code generated. One module was
specified by hand on 0.4.0; a pilot of `specarch extract` and `merge` on
a second ran on 0.6.0 and asked for twelve things. This file is the
answer: where each stands, how the planned ones are built and in which
order, and what the owner decides first.

Three tests are applied to every point. The compiler test
(`docs/principles.md`): does it keep an incomplete or wrong definition
readable, with every problem in the one problems file and marked at the
entry, and does a rule that refuses something refuse it at the entry
rather than leave a source unread? The acceptance test
(`docs/acceptance.md`): does it make a result fast to show, and a change
asked for by someone looking at it cheap to take back into the
specification? The readers' test (ADR-044, ADR-050, ADR-075): the rules
of `extract` stay in one place, in Go; a project writes a printer of
facts in a format SpecArch defines, never a reader of its own.

## Where each point stands

| Point | Standing |
|---|---|
| 1. a dump committed outside the code's repository | planned, item 1 |
| 2. references between specifications; an umbrella specification | planned, items 5 and 6 |
| 3. documents in another language | planned, item 4 |
| 4. role names on the wire | planned, item 2 |
| 5. authorisation by path pattern | planned, item 3: pattern rules in the permission table and `implies` on a permission; a permission defined by a pattern is declined |
| 6. a check bypassed for one kind of caller | planned as a gate kind in the permission table, item 7; whether a route may serve people and clients both is the owner's question 3 |
| 7. the schema file's name; the facts dump from the installed binary | planned, item 8 |
| 8. workflow definitions that are not BPMN | planned as a printed workflow table, item 9; a plug-in reader waits |
| 9. audience, requirements held elsewhere, material questions, partial answers, row citations | planned, items 10 and 11; a row citation is shipped in 0.6.0 as a clause that carries the row's identifier |
| 10. `--given-outside`, two routes for one list, generated defaults, indexes, a source key from capitals | planned, items 4 and 12; an index goes to the implementation file; the source key from capitals is on main already, after 0.6.0 |
| 11. generation targets | shipped: the OpenAPI document and SQL migrations (0.4.0), the Next.js UI with task pages and a private component library through an idiom override (0.6.0); declined: Go server interfaces, which the stack's OpenAPI code generator makes from the document; planned later: sqlc queries, item 13 |
| 12. output a CI gate compares with the code | planned later, item 14, on the readers and `merge` |

## Must have

### 1. A dump committed outside the code's repository

`extract database`, `router`, `permissions`, `swift`, `javascript` and
`dart`, and `extract pages --facts`, read a dump a printer made from
code. The dump names the `path` it was made from and the `commit` that
last changed that path; the reader checks that the dump is committed,
that the path is in the dump's own repository, that the commit is in
its history and that the path has not changed since, so that a dump is
never stale and the code source's `edition` is a real commit
(`docs/extraction.md`, the rules every reader shares). That check
assumes the dump is committed in the code's repository. A team that may
only read the code cannot commit anything there, and a local branch
holding the dumps, the pilot's way round, gives an edition that exists
on one machine only.

The dump gains an optional key, `repository`: the path of the code's
repository from the root of the repository holding the dump, a
submodule's folder in the usual case. With it the reader checks:

- the dump file itself is committed, in its own repository B, as today;
- `repository` is a folder in B that is a git repository root and a
  gitlink of B (`git ls-tree HEAD -- <repository>` gives mode 160000);
  the commit B records there is the reference commit of A. A folder that
  is a plain checkout and no gitlink is refused with that reason, since
  nothing in B then pins which commit of A was read;
- `commit` is in A's history, A is not a shallow clone, and `path` has
  not changed in A between `commit` and the reference commit
  (`git -C A log -1 --format=%H <reference> -- <path>` equals `commit`);
  otherwise the dump is stale, and the refusal names both commits and
  the script to run again.

The reference commit is the gitlink, not A's working checkout: a
checkout moved ahead or left dirty in A changes no answer, and two
machines with the same B commit give the same bytes. A's own working
tree is still refused when it has changes not committed under `path`,
as today, because what the printer read is then not what any commit
names. The code source's `edition` stays A's `commit`, its `url` points
at A's root as it does today, so `specarch merge` finds A to compare
editions, and B's commit of the dump appears nowhere in the tree: a
commit elsewhere in B leaves the output byte for byte as it was. Every
printer script (`dump-catalogue.sh`, `dump-routes.sh`,
`dump-permissions.sh`, the three `dump-<language>.sh`) takes the dump
file's place as it does now and writes `repository` by itself when the
dump file's repository is not the folder's; nothing is asked of the
project's own printer beyond the one key. A dump without `repository`
is read exactly as today.

Compiler test: a dump that fails a check is refused with the reason,
before anything is read, as every dump check is today; nothing is
silently taken from a wrong commit. Acceptance test: the extracting
team re-runs a printer and commits the dump in its own repository, so
a change on the code side reaches the specification without a change
request to the code's owners.

### 2. References between specifications, and the umbrella

One specification per module is the natural unit of an estate whose
modules different teams own. Then a table keeps the code of another
module's record with no foreign key, a page's picker reads another
module's list, and a requirement given to the customer is fulfilled by
another module (point 9). `$ref` reaches `#/entities/X`, `#/enums/X`,
`#/views/X` and `#/schemas/X` of the same specification only
(`docs/conventions.md`, Where each keyword comes from), and the one
cross-system form is `dependencies` with `calls`, which names a system
and not an element of it.

The reference takes JSON Schema's own form, since `$ref` is a URI
reference and the rule for new keywords says to take a standard's
keyword with its exact meaning: `<name>#/<pointer>`, where `<name>` is
the key of a dependency and the pointer is one of the four `$ref`
targets or an operation (`#/paths/~1items/get`). A dependency gains two
keys that make it a specification and not only a called system:

```yaml
dependencies:
  customerRegistry:
    description: The registry of customers, whose numbers this module's tables keep.
    timeout: PT2S
    specification: customer-registry
    version: 2.4.0
```

`specification` is the other specification's name, the kebab-case key
the umbrella lists it under; `version` is the `info.version` the
reference was written against. A relation whose `target` is
`customerRegistry#/entities/Customer`, a picker or a list page whose
`source` is `customerRegistry#/paths/~1customers/get`, a field whose
`$ref` is a schema of it, and a `calls` entry are each a reference. The
validator, given one specification alone, checks that `<name>` is a
dependency with `specification`, and counts the reference as unresolved
in the summary line, beside the open questions, with the list under
`specarch gaps`: the module validates with no errors on its own, which
is what one team needs. Given the umbrella, or the referenced
specification among its inputs, it resolves the pointer in that
specification and checks it as it checks a local one: the entity exists,
the operation answers a list where a list is read, the version matches
the other's `info.version`, and the referenced element is not itself
blocked by a must question, in which case the reference is covered as
the element is. A reference into a specification that is given and does
not hold the element is the error `reference` at the entry.

The umbrella is a root file of its own kind, `specarch-estate`, with no
stages: `info`, and `modules`, each the specification's key, its
`location` (a folder relative to the umbrella file, a module's `spec/`
folder in a submodule) and the `version` it holds. `specarch validate
<umbrella>` reads every module, checks that each `info.version` is the
one listed, and resolves every reference across them; `specarch gaps
<umbrella>` lists each module's open questions and unresolved
references in one report; `document techspec` on it writes the module
map and the references between modules as the context diagram. A module
listed and not found, a version that differs and a cycle of references
through `calls` are errors at the module's entry. Nothing of a module is
copied into the umbrella: it lists and pins.

Generators treat a reference as they treat an owned element (ADR-046):
`generate sql` writes the column that keeps the other module's key with
the type the referenced key has, and no foreign key, since the table is
in another database; `generate openapi` writes the referenced schema
inline as the other specification gives it, marked `x-specarch-ref`
with the specification and version; the UI generators read a picker's
remote list through the dependency's base URL, which the implementation
file maps as it maps every dependency. The techspec names each reference
with the module and version it points at.

Compiler test: a reference the validator cannot resolve is a counted,
listed gap and not an error, so a module alone is readable; an error
comes only when the other specification is given and contradicts the
reference, at the entry. Acceptance test: a module is shown on its own,
and the estate's consistency is checked where the estate is, in the
umbrella, in one run.

### 3. Documents in another language

`extract documents` finds a commitment by the English modal words and
reads a table of fields only under English headers; the sensitivity
words, the yes words of a Required column and the quantity words (days,
hours) are English too. A document in another language gives its
outline and nothing else, each table a `could` line.

The words become data. A vocabulary file holds, for one language, the
commitment words, the table headers (field, type, required, sensitivity,
description, and the row-number header), the yes words, the sensitivity
words that mean personal and credential, the quantity units with the
ISO 8601 duration each stands for, and how a time of day is written.
SpecArch ships the English vocabulary and reads it by default. A
source's `language` (a BCP 47 tag, `docs/conventions.md` gains the key
beside `givenOutside`) is a fact about the document and so design; the
reader takes `--language <tag>` and `--vocabulary <file>` and writes
`language` on the source, so a reader of the tree knows which words
were applied. A tag with no shipped vocabulary and no file is refused
with the format to write one in; a vocabulary is checked when read, and
a word in two lists is an error at the file. Which languages are
shipped beyond English is decided when the first project supplies a
vocabulary; a project's file is in its specification repository, not in
SpecArch.

One small rule goes with it, in every language: a leading column headed
with the row-number header (`#`, `No.` or the vocabulary's word) is
skipped in every table. The default source key of a file whose name is
a run of capitals (`ARCHITECTURE.md`) is that run as one word
(`architecture`) already, on main after 0.6.0 (issue #9), so two such
files no longer collide and `--source-key` is the exception again.

Compiler test: a table whose headers no vocabulary matches still prints
its `could` line and keeps the outline, as today; the vocabulary widens
what is read and refuses nothing. Acceptance test: a wrong word is one
line in a data file, changed without a release.

### 4. Role names on the wire

A role is `UPPER_SNAKE` in the token's roles claim and in the
permission table, and kebab-case in a specification; `extract
permissions` leaves out every grant whose role is not kebab-case, so a
module whose roles are all upper snake comes out with no role. A
property has the same split, settled by one rule for the whole contract,
`info.wireNames: snake_case` (ADR-062), rather than a name per property.

Roles take the same shape: `info.roleNames: upper_snake` says that
every role goes on the wire as its kebab-case name in upper snake
(`desk-supervisor` is `DESK_SUPERVISOR`). The validator refuses two
roles that map to one wire name (`wire_name`, the rule that already
holds for properties). `extract permissions` and `extract go` write
`roleNames: upper_snake` into the tree's `info` and map each name when
every role in the table is upper snake; a table with roles in both
forms, or a name that would not go back on the wire unchanged, keeps
today's `could` line for those roles. `generate openapi`,
`specarch-gen-go-dxlib`'s role data, the UI generators' session reading
and the techspec write the wire form where the wire is meant. The token
claims of meta-model 0.3 carry the roles in the same form, since a
claim is a property under the one rule (ADR-097).

Compiler test: a role the rule cannot map is still a line and a
question, never dropped. Acceptance test: one key in `info` answers it
for the whole estate.

### 5. Authorisation by path pattern

The check is a policy of rows, each a role, a path pattern and a method
or `*`, matched with the library's path matcher: `*` matches the rest
of the path, slashes included; one matcher also takes `:name` and
another `{name}` for one segment. No permission name exists anywhere in
the system. The pilot's printer made names by a rule of its own and had
to grant a role with a `*`-method row two names, since nothing says
that one permission covers another.

Two forms were asked for. A permission defined by a pattern and a set
of methods, so the route table's permission is computed, is declined:
rows overlap by design (`/v1/*` with `*` beside `/v1/*` with `get`), so
a computed permission gives one operation several permissions, where
the meta-model gives it exactly one and reads an ambiguity as an error;
and a permission that follows a pattern would reach a route added later
without anyone writing it, which is the fail-open that fail-closed
access exists to refuse (`docs/conventions.md`, Access control is
fail-closed). `implies` on a permission is taken:

```yaml
permissions:
  v1.all:
    description: Every call under /v1.
    implies: [v1.get]
  v1.get:
    description: Every read under /v1.
```

A permission implies the permissions it lists, transitively; a cycle
and an undeclared name are errors (`implies`). Wherever the validator
or a document asks what a role grants, it expands the implications
first: a permission is granted when a role grants it or one that
implies it; separation of duties counts implied permissions; the
techspec's matrix shows them. `implies` is design because the client
depends on it (a holder of `v1.all` calls a read without being told of
`v1.get`), which is the interface boundary's test.

The permission table gains `rules`, beside `grants` and `gates`: each a
role, a pattern, the methods (a list, or `*`) and the matcher's name.
`extract permissions`, given the route table too (`--routes`), writes
one permission per distinct pattern and method pair, named by a fixed
rule: the pattern's literal segments as lower-case words joined by
dots, `*` and a parameter dropped, then `all` for `*` or the method
(`/v1/*` with `*` is `v1.all`; `/v1/items/{id}` with `delete` is
`v1.items.delete`); a row listing several methods is one permission per
method. A permission implies another when its pattern covers every path
the other's covers, by the matcher's own rule, and its methods include
the other's; the implications are written. Each role is granted the
permissions of its rows. Each route's permission is the one matching
permission that every other matching permission implies; a route two
permissions match with neither implying the other is a `must` question
at the operation, and so is a route no row reaches (closed on purpose,
which 0.3 writes as `nobody`, or by accident). A rule no route matches
is a `could` question. Every name is inferred, with a `could` question
offering to rename, since the words come from paths and not from a
person.

Compiler test: ambiguity and a closed route are questions at the
operation; the permissions are written. Acceptance test: a reviewer
reads the matrix with the implications expanded and sees in one table
what each role reaches, which is what the rows never showed.

## Should have

### 6. A check that lets one kind of caller through

The role check skips a machine-to-machine token entirely: any service
token passes every route the role check guards, and the routes meant
for machines alone are guarded by a separate middleware. The permission
table has `gates` for a check a setting switches off; nothing says a
check switched off by the kind of caller.

The permission table's gate gains a second form: beside `setting`, a
gate may name `bypassedFor` with the kind of caller the check skips
(`client`, in 0.3's word). `extract permissions` writes it as it writes
a setting gate: a `must` question at every operation the check guards,
asking whether every client is meant to reach it, with the reason that
a check skipped for a caller kind fails open for that kind, and a line
in the output. `extract go` asks the same where the implementation
file's bindings name the check. That works on a 0.2 tree today and
needs no keyword.

What the meta-model says about it is 0.3's: a client holds roles, a
machine route is `callers: clients`, and a route both people and clients
call is in the left-out table because no system asked for one
(`docs/meta-model-0.3.md`). This system has one on every guarded route,
by accident or by design, which is exactly what the question asks the
owners. Whether 0.3 reopens that row, with a `callers: all` that the
validator reports as every client reaching the operation, is the
owner's question 3; until then the gate's question is the record. The
meta-model side of the calling service, and the three inputs beside it
(the application a person signed in through, the delegation chain and
the audience per hop), are designed in `docs/caller-context.md`, which
recommends `callers: all` for question 3.

### 7. The schema file's name, and the facts dump from the installed binary

The admin application keeps the schema its component library renders
from in `_schema.ts` beside `page.tsx`, a private file by the App
Router's rule, where `extract pages` reads `page.schema.ts` and
`specarch-gen-ui-typescript` writes it. The name is the project's
convention, so it goes in the implementation file under the UI
bindings, `bindings.ui.schemaFile`, beside `bindings.ui.components`;
`extract pages --implementation` reads that file where it reads
`page.schema.ts` today, and the generator writes its schemas under that
name, so a project that has the one convention gets both halves from
one key.

The JavaScript facts dump is made by `readers/javascript/`, a Node
program on the TypeScript compiler pinned in its lock file, which
`dump-javascript.sh` installs with `npm ci` inside the SpecArch
checkout. A team that installs the binary has no checkout. The reader
and its lock file are embedded in the binary, and `specarch facts
javascript <folder> <dump>` writes them once into a per-user cache
(`$XDG_CACHE_HOME/specarch/readers/javascript-<version>`), runs `npm ci`
there against the embedded lock file, whose integrity hashes pin every
byte, and runs the reader; the binary already refuses a dump another
compiler version made, so the pin cannot drift. Nothing is written into
the source tree or the specarch checkout, no `npx`, no unpinned
package. The same verb serves `swift` and `dart` from their toolchains.
The shell scripts stay for a checkout and call the verb.

### 8. Workflow definitions that are not BPMN

The workflow engine's definitions are JSON: a flow per action, a
trigger, a binding and a form schema, in many files, and the same files
sit in two folders of which the seeding script loads one. Only `extract
outline` takes them, so `gaps` lists the folder as unread, which is the
right answer and no more.

A documented plug-in reader was asked for, or a neutral format a
project printer writes. The format is taken, the pattern of the route
table (ADR-044) and the permission table (ADR-050): SpecArch defines
the workflow table, `workflowTable: 1`, the folder and the commit, and
every workflow as its name, description, trigger (the operation's name,
or null, and the form's fields with their types), its steps in order
(an approval with its roles, deadline and what happens on the deadline,
or an operation by name) and how it ends. The project's printer reads
what the engine loads, which answers the two-folders question by
construction, since the printer runs the seeding script's own choice.
`extract workflows --table` reads it into the 0.2 workflow object with
the questions the BPMN reader asks: the trigger's and each step's
operation left out with a `must` question carrying the name under
`names`, which `merge` fills from the router's tree; the approval's
permission and the subject asked for. A plug-in that writes elements
would put extract's rules in a second place and is the one thing the
readers' test forbids; a plug-in reader protocol waits until a format
appears that a printer cannot flatten.

### 9. Audience, requirements held elsewhere, material questions, partial answers, row citations

**Audience.** An element from an internal note must never reach the
customer's documents; today it is an extension key and a rule read by
people. `audience: internal` on any element says so; left out, an
element is for everyone. `specarch document --audience external` leaves
internal elements out and says in the document's summary how many it
left out, in each section; without the flag a document holds everything
and marks each internal element. A requirement written as a proposal
below the promises uses the same key. The keyword earns its place with
one check: an element for everyone may not depend on an internal one (a
relation to an internal entity, an operation whose body refers to an
internal schema, a requirement an internal requirement refines), which
is the error `audience` at the entry, because the external document
would otherwise name what it may not show. The key is SpecArch's, with
its row in the origin table; no standard of the stages has one.

**Requirements held elsewhere.** A promise in the customer's documents
that another component fulfils has no form: as a requirement the
validator asks for what satisfies and verifies it; left out, the trace
is lost. A requirement may carry `ownedBy`, the one word SpecArch
already has for what someone else builds (ADR-046), naming the
stakeholder whose component fulfils it; with a reference (point 2),
`fulfilledBy: <dependency>#/requirements/<id>` names the element in that
specification, and the umbrella checks it. The validator asks no
`satisfies`, no test and no design element for it, traceability lists it
under its own heading with the owner, and coverage counts it as read.
`ownedBy` moves from the implementation file into the requirements
stage for this one use because which component fulfils a promise is a
fact about the requirement, not about a stack.

**Material questions.** `kind: material` exists; `gaps` and the
`questions` document present a material question with the decision's
frame and count it among the decisions. Both gain a group of their own,
worded as what is needed and from whom (the question text, `decidedBy`),
and the count of decisions a stakeholder must take leaves them out. An
output change only.

**A decision that answers part of a question.** A decision closes the
questions it `answers`; a split answer wrote the open half in prose.
The convention is kept, close and open, and made visible: a decision
may name under `opens` the questions it raised, each checked to exist
(`question_opened`), and `gaps` shows the chain, the new question under
the decision that opened it and the question that decision answered.
One key, no new semantics.

**Row citations.** A citation's `clause` falls under the longest listed
clause it equals or starts with, followed by a dot, a colon, a slash or
a space (`docs/conventions.md`), so `clause: "4 F-13"` is a row of
clause 4 already, and that is the convention: the section, a space, the
row's identifier. What was missing is the check: a citation whose clause
falls under none of a source's listed clauses is the warning
`citation_clause`, the rule `docs/from-sources.md` kept for the first
real project to ask for it.

**Smaller.** A requirement's `release` names a release record, planned
or released; a proposal for a release nobody has planned is written by
planning it, one record with `status: planned`, so no "planned, no
record" value is added: the rule protects against a release named
nowhere, and that protection applies. A per-file acknowledgement of the
meta-model version warning is not added either: the warning is the one
signal to migrate, it is one line per file with the rule's id, and the
problems file is filtered by id. The outputs table of `gaps` shows, per
stage blocked, a count and the first identifiers rather than every one.

### 10. Smaller points of the readers

- `extract documents --given-outside` writes `givenOutside: true` on the
  source, so a document given to the customer gets its priority in
  `merge` without an edit by hand.
- A handler that serves a GET on a path and a POST on that path with a
  literal last segment (`/items` and `/items/query`), the standard list
  contract of this estate, is two operations in OpenAPI and stays two;
  what goes is the noise: `extract router` names them after the handler
  with the method, cites both and asks no question, and the
  implementation file may say under `bindings.http.listRoutes` that
  every list is served both ways, so `generate openapi` writes the twin
  for each `listOf` operation. The convention is HTTP's, so it is the
  implementation file's.
- A column default the database computes (`gen_random_uuid()`, `now()`)
  is not a value, so `default` cannot hold it; today each is a `could`
  line. A field gains `generatedDefault`, a closed set the stacks
  share, `identifier` and `currentTime`; the type-rendering idiom maps
  each to the dialect's expression, `generate sql` writes it, and the
  readers write it where the catalogue's expression is one the idiom
  names. The list stays closed: an expression the idiom does not name
  is still a `could` line.
- A plain index is physical design, the implementation file's, as
  `docs/sync-gates.md` already says. `mappings.<entity>.indexes` lists
  each as its fields, the SQL generator writes them, the database gate
  compares them, and `extract database` writes them, which is the first
  thing a reader writes into an implementation fragment: `--implementation-out`
  on the database reader, a fragment holding the mappings it knows. The
  owner's question 6 is whether a reader may write one.

## Needed for the later phases

### 11. Generation targets for this stack

- Go server interfaces and types from the OpenAPI document: declined as
  a SpecArch target, by `docs/generators.md`'s own rule that where a
  standard tool exists for a step the emitter stops at that tool's
  input. `specarch generate openapi` writes the document; the stack's
  OpenAPI code generator in its strict server mode writes the interface
  and the types, and hand-written handlers implement it. The one thing
  SpecArch adds is the generator's configuration file as a setting of
  the `openapi` target, so its layout is in the specification.
- Versioned SQL migrations: shipped in 0.4.0, `specarch-gen-sql`,
  PostgreSQL among its dialects, new files only, expand and contract,
  with the snapshot and `--check`.
- sqlc queries from the data model: a plausible later plug-in,
  `specarch-gen-sqlc`, writing the queries SpecArch can derive, one per
  operation that reads or writes an entity by its key and one per list
  with its filters and sorts, and leaving every other query to be
  written by hand, which is sqlc's own rule. Item 13, after a real
  project's first hand-written query file to check against.
- A Next.js UI generator writing schema files for a private component
  library: shipped in 0.6.0, `specarch-gen-ui-typescript`. It writes
  task pages (sign-in and a second factor are its checked screens; a
  password reset is a task page too), list, form and view pages, each
  as a data-only schema file and a `page.tsx`, the server routes,
  strings per language and the theme; a private component library is
  an override of the `ui-components` idiom naming its package under
  `import`, and a project whose schema shape the idiom's names cannot
  say installs its own plug-in ahead of the shipped one. Of the earlier
  UI request, `docs/ui-nextjs-carbon.md` is the plan and all seven of
  its steps are built, the idiom, task forms, lists, forms and views,
  child rows and approvals, server routes with strings and theme, and
  the comparison with screens built by hand; left out there: rows in a dialog and a page that follows
  an approval to its end (ADR-071). Point 7 adds the schema file's
  name.

### 12. Output a CI gate compares with the code

The three comparisons are designed (`docs/sync-gates.md`), and the
pieces exist: the route and permission tables and the catalogue are
read by the readers, `merge` finds every difference between a tree
and another, and `generate --check` is gate 3. What is missing is a
verb that exits 1 on a difference and takes an allowlist that may only
shrink. `specarch check <specification> <dump>...` reads each dump with
its reader, compares the tree with the specification by the merge's
own matching (an operation by method and path, a field by entity and
name, a grant by role and permission), prints each difference as a
problem at its entry and exits 1 unless the allowlist names it; a line
in the allowlist that no difference matches is itself a failure, so the
list shrinks. Exact comparisons fail the build; the fuzzy ones (a check
expression's text, an index the mapping does not list) are reports,
after the gates' rule. Item 14.

## Declined

| Asked | Why not |
|---|---|
| a permission defined by a path pattern and methods | overlapping rows give one operation several permissions, where it has exactly one and an ambiguity is an error; a permission that follows a pattern reaches a route added later without anyone writing it (point 5) |
| a plug-in reader for a workflow format | extract's rules stay in one place; a printer writes a workflow table and the reader reads it (point 8) |
| a shipped vocabulary per language before a project supplies one | the words are a project's to check; English is shipped and the format documented (point 3) |
| `release` naming a release with no record | the record is one file with `status: planned`; the rule protects against a release named nowhere (point 9) |
| a per-file acknowledgement of the version warning | the warning is the one signal to migrate and is filtered by its id (point 9) |
| Go server interfaces as a SpecArch target | the stack's OpenAPI code generator makes them from the document SpecArch writes (point 11) |
| an index as a design keyword | an index is physical design and goes in the implementation file's mapping (point 10) |

## Building

Each item changes `spec/` first, adds its conformance cases, writes its
decision record, numbered when it lands, and appends to `history/`; an item that touches the
validator is made in both builds with the same output. Items 1 to 6
answer the must-have points and go first; every keyword among them is
an addition that breaks no 0.2 file, so none waits for meta-model 0.3
step 1. Only item 7's meta-model half touches 0.3.

1. `repository` in every dump. The key in the catalogue, route, permission
   and code-facts formats; the gitlink checks in `openDump`; the six
   printer scripts writing it; the conformance case with two throwaway
   repositories, one a submodule of the other, read at a fixed gitlink.
   Done when a dump committed in the outer repository reads the inner
   one's code at the gitlink's commit, byte for byte with the dump
   committed inside, a stale gitlink is refused naming both commits, and
   a dump without the key reads as today.
2. `info.roleNames: upper_snake`. The key in the design schema, the
   `wire_name` rule for roles in both builds, the two readers mapping
   and writing it, the generators and the techspec writing the wire
   form. Done when the permission-table case with upper-snake roles
   gives every role, two roles mapping to one wire name are refused in
   both builds, and the techspec shows the wire form.
3. `implies` and pattern rules. `implies` in the design schema with the
   `implies` rule (cycle, undeclared) in both builds, expansion in the
   grants check, separation of duties and the techspec matrix; `rules`
   in the permission table with `--routes` on the reader, the naming
   rule, the implications from pattern inclusion by matcher, the
   questions for an ambiguous route, a closed route and a dead rule.
   Done when a conformance case with a `*` row beside a `get` row gives
   two permissions with the implication, each route its one permission,
   and the lending desk's matrix is unchanged.
4. Documents in another language, and the small reader points. The
   vocabulary format and the English file, `language` on a source,
   `--language` and `--vocabulary`, the row-number column,
   `--given-outside`. Done when a conformance document in a
   second vocabulary gives the same elements the English one does, a
   file with a word in two lists is refused, and the English run is
   byte-identical with today's.
5. References between specifications. `specification` and `version` on
   a dependency, `<name>#/<pointer>` wherever a `$ref`, a `target`, a
   `source` or `calls` may stand, the `reference` rule in both builds,
   the unresolved count in the summary and the list in `gaps`, the
   generators' handling, the techspec's naming. Done when a module alone
   validates with its references counted, two modules given together
   resolve them, a wrong pointer is an error at the entry in both
   builds, and `generate sql` writes the column with no foreign key.
  
6. The umbrella. The `specarch-estate` root with `modules`, `validate`,
   `gaps` and `document techspec` on it, the version check and the
   cycle check. `examples/` gains a two-module estate, the lending desk
   and a small catalogue service it references. Done when the example
   validates as an estate, a version that differs and a module not
   found are errors at the module, and the techspec draws the module
   map.
7. The caller-kind gate. `bypassedFor` in the permission table's gates,
   the question in both readers, and, after the owner's answer to
   question 3, the 0.3 plan amended or its left-out row confirmed.
   Done when the permission-table case with a bypassed check asks at
   every guarded operation. The 0.3 decision is recorded with the
   amendment, if any.
8. The schema file's name and `specarch facts`. `bindings.ui.schemaFile`
   read by `extract pages` and written by the UI generator; the readers
   and lock files embedded, the per-user cache, the verb for the three
   languages, the scripts calling it. Done when the lending desk's web
   folder is dumped by the installed binary with no checkout and
   `extract pages --facts` reads it byte for byte as today's dump, and a
   page whose schema is `_schema.ts` is read through the binding.
  
9. The workflow table. The format, `extract workflows --table`, and a
   conformance case whose table gives the write-off workflow the BPMN
   case gives. Done when both readings merge into one workflow with
   both citations.
10. `audience`, `ownedBy` on a requirement, `fulfilledBy`. The keys in
    the schemas, the `audience` dependency rule and the owned
    requirement's exemptions in both builds, `document --audience`, the
    traceability heading. Done when an external document leaves the
    internal elements out and counts them, an everyone element
    depending on an internal one is refused in both builds, and an
    owned requirement draws no satisfied or verified warning.
11. Questions in the outputs. Material questions grouped, `opens` on a
    decision with its rule, the chain in `gaps`, the per-stage count,
    `citation_clause`. Done when the lending desk's `gaps` and
    `questions` show the groups and a misspelt clause warns in both
    builds.
12. The list twin, `generatedDefault`, indexes in the mapping. The router
    reader's naming, `bindings.http.listRoutes` in the OpenAPI
    generator; `generatedDefault` in the schema, the idiom rows, the SQL
    generator and the readers; `mappings.<entity>.indexes` and
    `--implementation-out` on the database reader, after the owner's
    question 6. Done when the lending desk's catalogue reads its
    identifier default and indexes with no `could` line for them and
    `generate sql --check` passes.
13. sqlc queries, `specarch-gen-sqlc`, checked against a real project's
    first hand-written query file.
14. `specarch check` with the shrinking allowlist, run in the lending
    desk's CI.

## For the owner to decide

The recommendation is first in each.

1. **Pattern rows become `implies` on permissions, one per distinct
   pattern and method pair, named by the fixed rule**, or one
   permission per route named after its operation with the grants
   computed and no `implies`. Recommended: `implies`. It gives a matrix
   a person reads, few permissions with meaningful names, and the
   implication is a fact the client depends on; a permission per route
   gives one per endpoint and a change for every route added.
2. **A reference is `<dependency>#/<pointer>` through `dependencies`**,
   or a new keyword beside `$ref`. Recommended: through `$ref`'s own URI
   form, because the rule for new keywords says to take the standard's,
   and a dependency already is the other system.
3. **A route both people and clients call stays left out of 0.3, and
   the bypassed check is the gate's question**, or 0.3 gains
   `callers: all` reported as every client reaching the operation.
   Recommended: stay left out until the owners of such a system answer
   the gate's question; what they answer decides whether it is a design
   or a defect.
4. **The JavaScript reader is embedded and installed once into a
   per-user cache by `specarch facts`**, or shipped as a release
   tarball with its packages for the person to unpack. Recommended: the
   cache, since the binary knows the pinned version and `npm ci` against
   the embedded lock file pins every byte; the tarball adds a download
   and a checksum the person checks by hand.
5. **A workflow table the project's printer writes**, or a plug-in
   reader protocol. Recommended: the table, the pattern every printed
   surface follows.
6. **A reader may write an implementation fragment** (the database
   reader's indexes under `mappings`), or indexes stay a `could` line
   until the implementation file is written by hand. Recommended: the
   fragment, under `--implementation-out`, since the mapping is as
   mechanical as the entity and the procedure of `docs/from-sources.md`
   section 3.4 writes it from the code anyway.
7. **`ownedBy` on a requirement in the requirements stage**, or a
   `status` value for a requirement held elsewhere. Recommended:
   `ownedBy`, one word for one meaning; `status` keeps 29148's life
   cycle.
