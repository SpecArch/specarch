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
order, and what the owner decided.

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
| 1. a dump committed outside the code's repository | built, item 1 (ADR-098): `repository` in every dump, read at the gitlink's commit |
| 2. references between specifications; an umbrella specification | references built, item 5 (ADR-112): `info.name`, `specification` and `version` on a dependency, `<dependency>#/<pointer>`, resolved when the other specification is given in the same run and counted as unresolved otherwise; the umbrella built, item 6 (ADR-113): `specarch-estate.yaml` lists each module by `info.name` with its folder and version, brings every module into validate, gaps and document, checks each against its entry, and `document techspec` draws the module map |
| 3. documents in another language | built, item 4 (ADR-105): a vocabulary per language, English shipped, `--language` and `--vocabulary` on extract documents and `--vocabulary` on merge, `language` on a source, the row-number column skipped |
| 4. role names on the wire | built, item 2 (ADR-099): `info.roleNames: upper_snake`, read by both readers of grants |
| 5. authorisation by path pattern | built, item 3 (ADR-100): `implies` on a permission, counted by every check of a role's grants, and `rules` in the permission table, read with the route table by `extract permissions --routes`; a permission defined by a pattern is declined |
| 6. a check bypassed for one kind of caller | built, item 7 (ADR-114): `bypassedFor: client` on a gate of the permission table and on a check the implementation file names, one `must` question at every operation the check guards, joined by `merge`; a route that serves people and clients both lists them in `callers` (`docs/caller-context.md`, ADR-110) |
| 7. the schema file's name; the facts dump from the installed binary | built, item 8: `bindings.ui.schemaFile`, read by `extract pages` and written by `specarch-gen-ui-typescript` (ADR-115); `specarch facts` for JavaScript, Swift and Dart, the JavaScript reader inside the binary on typescript-go with no Node (ADR-116) |
| 8. workflow definitions that are not BPMN | built, item 9 (ADR-118): the workflow table, `workflowTable: 1`, printed by the project's printer through `tools/workflows/dump-workflows.sh` and read by `extract workflows --table` with the BPMN reader's questions at the same keys, so `merge` joins the two readings of one workflow; a standard format gets a reader of its own (decision 5) |
| 9. audience, requirements held elsewhere, material questions, partial answers, row citations | audience and requirements held elsewhere built, item 10 (ADR-119): `audience: internal` on any element named by a key, the error `audience` where an element for everyone names an internal one, `document --audience external --out` leaving the internal elements out and counting them, and `ownedBy` with `fulfilledBy` on a requirement, exempt from the satisfied and verified warnings and listed under Requirements held elsewhere; the material questions in a group of their own, `opens` on a decision with the rule `question_opened` and the chain in `gaps`, and the outputs table counting questions per stage built, item 11 (ADR-120); a row citation is shipped in 0.6.0 as a clause that carries the row's identifier |
| 10. `--given-outside`, two routes for one list, generated defaults, indexes, a source key from capitals | `--given-outside` built, item 4 (ADR-105); two routes for one list, generated defaults and indexes built, item 12 (ADR-121), an index in the implementation file; the source key from capitals is on main already, after 0.6.0 |
| 11. generation targets | shipped: the OpenAPI document and SQL migrations (0.4.0), the Next.js UI with task pages and a private component library through an idiom override (0.6.0); declined: Go server interfaces, which the stack's OpenAPI code generator makes from the document; sqlc queries built, item 13 (ADR-123), in PostgreSQL, checked against the lending desk's queries written by hand, the check against a real project's first query file still owed |
| 12. output a CI gate compares with the code | built, item 14 (ADR-124): `specarch check <specification> <dump>...` compares the route table, the permission table and the catalogue with the specification, the catalogue through the tables `generate sql` writes, prints each difference at its entry with an id, and fails on one the allowlist does not name; a line of the allowlist that allows nothing fails, and with `--since` so does one added since that revision |

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

The dump takes an optional key, `repository` (ADR-098): the path of the
code's repository from the root of the repository holding the dump, a
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
- A's checkout holds the reference commit's files under `path` (the same
  tree object); otherwise it is refused with the `git submodule update`
  that checks the reference out. A gitlink not checked out, and a
  reference commit A's history does not hold, are refused with that
  reason.

The reference commit is the gitlink, not A's working checkout: a
checkout moved ahead without touching `path` reads as the reference,
one that changed `path` is refused rather than read at another commit,
and two machines with the same B commit give the same bytes. A's own working
tree is still refused when it has changes not committed under `path`,
as today, because what the printer read is then not what any commit
names. The code source's `edition` stays A's `commit`, its `url` points
at A's root as it does today, so `specarch merge` finds A to compare
editions, and B's commit of the dump appears nowhere in the tree: a
commit elsewhere in B leaves the output byte for byte as it was. Every
printer script (`dump-catalogue.sh`, `dump-routes.sh`,
`dump-permissions.sh`, `dump-workflows.sh`, the three `dump-<language>.sh`) takes the dump
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

`specification` is the other specification's name, its `info.name` in
kebab case, which the umbrella lists it under; `version` is the `info.version` the
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

The umbrella is a file of its own kind, `specarch-estate.yaml`, with no
stages: `info`, and `modules`, each keyed by the module specification's
`info.name`, with its `location` (a folder relative to the umbrella file,
a module's `spec/` folder in a submodule) and the `version` it holds.
`specarch validate <umbrella>` reads every module, checks that each
`info.name` is its key and each `info.version` the one listed, and
resolves every reference across them; `specarch gaps <umbrella>` prints
the umbrella's problems and then each module's open questions and
unresolved references; `document techspec` on it writes `estate.md`, the
module map, with the references between modules as the context diagram.
A module listed and not found, a name or version that differs, and a
cycle of calls between modules are errors at the module's entry. Nothing
of a module is copied into the umbrella: it lists and pins.

Generators read one specification at a time: `generate sql` writes the
column that keeps the other module's key with the type of its own `via`
field, and no foreign key, since the table is in another database, with
a comment naming the specification and version. A field typed by the
other specification's element, and for the UI a page or picker reading
its list, cannot be written from one specification alone, so each code
target refuses it at the field as `reference`; writing the referenced
schema inline, marked `x-specarch-ref`, and a picker's remote list
through the dependency's base URL wait for generation across the
estate, where the other specification is read too. The techspec names
each reference with the module and version it points at.

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
a word in two lists of one group (two priorities, two headers, two
sensitivities, two types) is an error at the file. The vocabulary also
gives the type words of a Type column and the linking and leading words
merge reads a requirement's subject by, and merge reads a number of
days by the vocabulary of its source's language (`docs/vocabularies.md`). Which languages are
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
permission table, and kebab-case in a specification; a reader that took
only kebab-case roles would leave out every grant of a module whose
roles are all upper snake, and the module would come out with no role. A
property has the same split, settled by one rule for the whole contract,
`info.wireNames: snake_case` (ADR-062), rather than a name per property.

Roles take the same shape: `info.roleNames: upper_snake` says that
every role goes on the wire as its kebab-case name in upper snake
(`desk-supervisor` is `DESK_SUPERVISOR`). The validator refuses two
roles that map to one wire name (`wire_name`, the rule that already
holds for properties). `extract permissions` and `extract go` write
`roleNames: upper_snake` into the tree's `info` and map each name when
no role in the table is kebab-case and one at least is upper snake; a
table with roles in both forms, or a name that would not go back on the
wire unchanged, keeps the `could` line for those roles, with the
reason. `specarch-gen-go-dxlib`'s role data and the techspec write the
wire form; the OpenAPI document and the UI generators' session carry
permissions and no role, so the wire form has no place there. The token
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
role, a pattern, the methods (a list, or `*`) and the matcher's name,
`colon` for the matcher that takes `:name` for one segment and `brace`
for the one that takes `{name}`.
`extract permissions`, given the route table too (`--routes`), writes
one permission per distinct pattern and method pair, named by a fixed
rule: the pattern's literal segments as lower-case words joined by
dots, `*` and a parameter dropped, then `all` for `*` or the method
(`/v1/*` with `*` is `v1.all`; `/v1/items/{id}` with `delete` is
`v1.items.delete`); a row listing several methods is one permission per
method. Two pairs the rule gives one name, such as the list and the
read-one route of `/v1/items` and `/v1/items/{id}` with `get`, are each
named with their parameters' words too (`v1.items.get` and
`v1.items.id.get`). With `--routes` the routes are written in the same
tree, as `extract router` writes them, each with its permission. A permission implies another when its pattern covers every path
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

The permission table's gate has a second form (ADR-114): beside
`setting`, a gate may name `bypassedFor` with the kind of caller the
check skips (`client`, in 0.3's word). `extract permissions` reads it as
it reads a setting gate, with a line in the output, and asks one `must`
question blocking the permission of every operation the check guards
(given the route table, each route that names a permission other than
`public`, or every route where the check reads rules), asking whether
every client is meant to reach them, with the reason that a check
skipped for a caller kind fails open for that kind. `extract go` asks
the same, in the same words, where the implementation file's
`permissionChecks` item names the check with `bypassedFor: client`,
since a test of the caller's token has no shape syntax finds; `merge`
asks it once with both citations. That works on a 0.2 tree and needs no
design keyword.

What the meta-model says about it is 0.3's as `docs/caller-context.md`
amends it: a client holds roles, and `callers` on an operation is the
explicit list of who may call it, `people` and clients by name, with no
`all` (ADR-110). This system lets every client through on every guarded
route, by accident or by design, which is exactly what the question asks
the owners; the gate's question is the record, and the answer, where it
is a design, is the client's name in that operation's `callers`, which
the validator refuses unless one of the client's roles grants the
permission. The meta-model side of the calling service, and the three
inputs beside it (the application a person signed in through, the
delegation chain and the audience per hop), are designed in
`docs/caller-context.md`, decided on 2026-10-14 (ADR-106 to ADR-111).

### 7. The schema file's name, and the facts dump from the installed binary

The admin application keeps the schema its component library renders
from in `_schema.ts` beside `page.tsx`, a private file by the App
Router's rule, where `extract pages` reads `page.schema.ts` and
`specarch-gen-ui-typescript` writes it. The name is the project's
convention, so it goes in the implementation file under the UI
bindings, `bindings.ui.schemaFile`, beside `bindings.ui.components`;
`extract pages --implementation` reads a page's schema from the file of
that name, and the generator writes its schemas under it, so a project
that has the one convention gets both halves from one key (ADR-115).

A team that installs the binary has no checkout of SpecArch, so the
readers come with the binary: `specarch facts <language> <folder>
<dump>` writes the dump (ADR-116). The JavaScript reader is inside the
binary, on the TypeScript compiler's own Go source (typescript-go,
copied into `third_party/typescript-go` at a pinned tag, since upstream
offers no public Go API), with a Vue reader of its own, so it needs no
Node, no npm and no cache. The Swift and Dart readers need their own
toolchains, for which no faithful Go parser exists: the binary carries
their source with the lock file that pins every dependency, writes them
once into a per-user cache named by a hash of those files, and builds
them there. The binary refuses a dump another reader version made, so
the pin cannot drift. Nothing is written into the source tree, no
`npx`, no unpinned package. The shell scripts stay for a checkout and
call the verb.

### 8. Workflow definitions that are not BPMN

The workflow engine's definitions are JSON: a flow per action, a
trigger, a binding and a form schema, in many files, and the same files
sit in two folders of which the seeding script loads one. Only `extract
outline` takes them, so `gaps` lists the folder as unread, which is the
right answer and no more.

A documented plug-in reader was asked for, or a neutral format a
project printer writes. The format is taken, the pattern of the route
table (ADR-044) and the permission table (ADR-050): SpecArch defines
the workflow table (ADR-118), `workflowTable: 1`, the folder and the
commit, and every workflow as its name, its file, its description, its
trigger (the operation's name, or null, and the form's fields with
their types), its steps in order (an approval with its approvers,
deadline, `onDeadline` and `escalateTo`, an operation step by its
operation's name, or a step of kind `other` the subset does not hold)
and how it ends, the messages it emits. The project's printer reads
what the engine loads, which answers the two-folders question by
construction, since the printer runs the seeding script's own choice.
`extract workflows --table` reads it into the 0.2 workflow object with
the questions the BPMN reader asks: the trigger's and each step's
operation left out with a `must` question carrying the name under
`names`, which `merge` fills from the router's tree; the approval's
permission and the subject asked for; the form's fields and the
messages emitted, which a 0.2 workflow has no key for, are each a line
and a `could` question. A plug-in that writes elements
would put extract's rules in a second place and is the one thing the
readers' test forbids; a plug-in reader protocol waits until a format
appears that a printer cannot flatten.

### 9. Audience, requirements held elsewhere, material questions, partial answers, row citations

**Audience.** An element from an internal note must never reach the
customer's documents, and an extension key with a rule read by people
does not keep it out. `audience: internal` on any element named by a key
says so; left out, an element is for everyone. `specarch document
--audience external --out <folder>` leaves internal elements out, with
the links, open questions and problems that concern only them, and says
in the document's summary how many it left out, in each section; without
the flag a document holds everything and marks each internal element. A
requirement written as a proposal below the promises uses the same key.
The keyword earns its place with one check: an element for everyone may
not depend on an internal one (a `$ref` to an internal entity, enum,
view or schema, such as an operation's body, a relation's target or a
page's entity that is internal, a requirement that refines an internal
need), which is the error `audience` at the name, because the external
document would otherwise name what it may not show. A `satisfies` or
`verifies` entry naming an internal requirement only traces, and the
external document drops it with the requirement. The key is SpecArch's, with
its row in the origin table; no standard of the stages has one.

**Requirements held elsewhere.** A promise in the customer's documents
that another component fulfils has no form: as a requirement the
validator asks for what satisfies and verifies it; left out, the trace
is lost. A requirement may carry `ownedBy`, the one word SpecArch
already has for what someone else builds (ADR-046), naming the
stakeholder whose component fulfils it; with a reference (point 2),
`fulfilledBy: <dependency>#/requirements/<id>` names the element in that
specification, and the umbrella checks it. The validator asks no
`satisfies`, no test and no design element for it (it still asks for
acceptance criteria), traceability lists it under its own heading with
the owner, and coverage counts it as read through its citations.
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
row's identifier. The check is the warning `source_clause`: a citation
whose clause falls under none of a source's listed clauses, or that names
no clause of a source that lists them.

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
  what goes is the noise: `extract router` names them after the handler,
  the POST's with `Post` added, cites both on each and asks no question
  about their names, and the implementation file may say under
  `bindings.http.listRoutes` that every list is served both ways, so
  `generate openapi` writes the twin for each `listOf` operation. The
  convention is HTTP's, so it is the implementation file's (ADR-121).
- A column default the database computes (`gen_random_uuid()`,
  `CURRENT_TIMESTAMP`) is not a value, so `default` cannot hold it. A
  field says `generatedDefault`, a closed set the stacks share,
  `identifier` and `currentTime`; the type-rendering idiom maps each to
  the dialect's expression, `generate sql` writes it, and the database
  reader writes it where the catalogue's expression is one the idiom
  names. The list stays closed: an expression the idiom does not name,
  such as `now()`, is still a `could` line.
- A plain index is physical design, the implementation file's, as
  `docs/sync-gates.md` says. `mappings.<entity>.indexes` lists each as
  its fields, the SQL generator writes them, the database gate compares
  them, and `extract database` writes them, which is the first thing a
  reader writes into an implementation fragment: `--implementation-out`
  on the database reader, a fragment holding the mappings it knows. By
  decision 6 a reader may write one.

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
- sqlc queries from the data model: `specarch-gen-sqlc` (ADR-123),
  writing the queries SpecArch can derive by keyword, an insert of an
  entity, a read, replacement or amendment by a key of one field, and a
  page of each list with its filters, search and sorts and its total, in
  PostgreSQL, against the tables `specarch-gen-sql` writes, and leaving
  every other query to be written by hand, which is sqlc's own rule
  (`docs/generators.md`, Queries for sqlc). A query file written by hand
  is compared, never written over. No real project's query file existed
  to check it against, so it is checked against the lending desk's,
  written by hand for this (decided 2026-10-14); a real project's first
  file is still owed, and decides whether `--check` needs to read the
  queries rather than compare their text.
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
after the gates' rule. Built, item 14 (ADR-124). The database is
compared through the tables `specarch generate sql` writes rather than the
database reader's entities, which are named after their tables and read a
CHAR column as a string of its width; `--since <revision>` makes a line
added to the allowlist fail, as `docs/sync-gates.md` asks.

## Declined

| Asked | Why not |
|---|---|
| a permission defined by a path pattern and methods | overlapping rows give one operation several permissions, where it has exactly one and an ambiguity is an error; a permission that follows a pattern reaches a route added later without anyone writing it (point 5) |
| a plug-in reader for a custom workflow format | extract's rules stay in one place; a printer writes a workflow table and the reader reads it (point 8); a standard format gets a reader (decision 5) |
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

1. Built (ADR-098). `repository` in every dump. The key in the catalogue, route, permission
   and code-facts formats; the gitlink checks in `openDump`; the six
   printer scripts writing it; the conformance case with two throwaway
   repositories, one a submodule of the other, read at a fixed gitlink.
   Done when a dump committed in the outer repository reads the inner
   one's code at the gitlink's commit, byte for byte with the dump
   committed inside, a stale gitlink is refused naming both commits, and
   a dump without the key reads as today.
2. Built (ADR-099). `info.roleNames: upper_snake`. The key in the design schema, the
   `wire_name` rule for roles in both builds, the two readers mapping
   and writing it, the generators and the techspec writing the wire
   form. Done when the permission-table case with upper-snake roles
   gives every role, two roles mapping to one wire name are refused in
   both builds, and the techspec shows the wire form.
3. Built (ADR-100). `implies` and pattern rules. `implies` in the design schema with the
   `implies` rule (cycle, undeclared) in both builds, expansion in the
   grants check, separation of duties and the techspec matrix; `rules`
   in the permission table with `--routes` on the reader, the naming
   rule, the implications from pattern inclusion by matcher, the
   questions for an ambiguous route, a closed route and a dead rule.
   Done when a conformance case with a `*` row beside a `get` row gives
   two permissions with the implication, each route its one permission,
   and the lending desk's matrix is unchanged.
4. Built (ADR-105). Documents in another language, and the small reader points. The
   vocabulary format and the English file, `language` on a source,
   `--language` and `--vocabulary`, the row-number column,
   `--given-outside`. Done when a conformance document in a
   second vocabulary gives the same elements the English one does, a
   file with a word in two lists is refused, and the English run is
   byte-identical with today's.
5. Built (ADR-112). References between specifications. `specification` and `version` on
   a dependency, `<name>#/<pointer>` wherever a `$ref`, a `target`, a
   `source` or `calls` may stand, the `reference` rule in both builds,
   the unresolved count in the summary and the list in `gaps`, the
   generators' handling, the techspec's naming. Done when a module alone
   validates with its references counted, two modules given together
   resolve them, a wrong pointer is an error at the entry in both
   builds, and `generate sql` writes the column with no foreign key.
  
6. Built (ADR-113). The umbrella. The `specarch-estate` root with `modules`, `validate`,
   `gaps` and `document techspec` on it, the version check, the check
   that each module's key is its `info.name`, and the cycle check.
   `examples/` gains a two-module estate, the lending desk
   and a small catalogue service it references. Done when the example
   validates as an estate, a version that differs and a module not
   found are errors at the module, and the techspec draws the module
   map.
7. Built (ADR-114). The caller-kind gate. `bypassedFor` in the permission table's gates,
   the question in both readers, and the 0.3 plan's left-out row
   answered (decision 3).
   Done when the permission-table case with a bypassed check asks at
   every guarded operation. The 0.3 decision is recorded with the
   amendment, if any.
8. Built (ADR-115, ADR-116). The schema file's name and `specarch facts`. `bindings.ui.schemaFile`
   read by `extract pages` and written by the UI generator; the
   JavaScript reader inside the binary on typescript-go, the Swift and
   Dart readers and lock files embedded with the per-user cache, the
   verb for the three languages, the scripts calling it. Done when the lending desk's web
   folder is dumped by the installed binary with no checkout and
   `extract pages --facts` reads it byte for byte as today's dump, and a
   page whose schema is `_schema.ts` is read through the binding.
  
9. Built (ADR-118). The workflow table. The format, `extract workflows --table`, and a
   conformance case whose table gives the write-off workflow the BPMN
   case gives. Done when both readings merge into one workflow with
   both citations.
10. Built (ADR-119). `audience`, `ownedBy` on a requirement, `fulfilledBy`. The keys in
    the schemas, the `audience` dependency rule and the owned
    requirement's exemptions in both builds, `document --audience`, the
    traceability heading. Done when an external document leaves the
    internal elements out and counts them, an everyone element
    depending on an internal one is refused in both builds, and an
    owned requirement draws no satisfied or verified warning.
11. Built (ADR-120). Questions in the outputs. Material questions grouped, `opens` on a
    decision with its rule, the chain in `gaps`, the per-stage count;
    the clause check shipped in 0.6.2 as `source_clause`. Done when the lending desk's `gaps` and
    `questions` show the groups and a misspelt clause warns in both
    builds.
12. Built (ADR-121). The list twin, `generatedDefault`, indexes in the mapping. The router
    reader's naming, `bindings.http.listRoutes` in the OpenAPI
    generator; `generatedDefault` in the schema, the idiom rows, the SQL
    generator and the readers; `mappings.<entity>.indexes` and
    `--implementation-out` on the database reader (decision 6). Done when the lending desk's catalogue reads its
    identifier default and indexes with no `could` line for them and
    `generate sql --check` passes.
13. Built (ADR-123). sqlc queries, `specarch-gen-sqlc`: the derived
    queries in PostgreSQL through `specarch-gen-sql`'s tables, the
    refusal to write over a file built by hand, `--check` against it, and
    `tools/sqlc/compile-queries.sh`. Done when, by `docs/generators.md`'s
    Adding a target, the output is what the project writes by hand, its
    `--check` form runs in the project's CI and sqlc compiles both: the
    lending desk's queries written by hand differ from the generated ones
    only at the phone number Q-6 leaves open, and the library's and the
    desk's queries compile. The check against a real project's first
    hand-written query file is still owed.
14. Built (ADR-124). `specarch check` with the shrinking allowlist,
    run in the lending desk's CI. Done when the desk's route table,
    permission table and catalogue pass with its allowlist, each line saying
    why it stays open, a line no difference matches fails,
    and a line added since `--since` fails.

## The owner's decisions

Decided on 2026-10-14.

1. **Pattern rows become `implies` on permissions, one per distinct
   pattern and method pair, named by the fixed rule**, rather than one
   permission per route named after its operation with the grants
   computed. It gives a matrix a person reads, few permissions with
   meaningful names, and the implication is a fact the client depends
   on; a permission per route gives one per endpoint and a change for
   every route added.
2. **A reference is `<dependency>#/<pointer>` through `dependencies`**,
   in `$ref`'s own URI form rather than a new keyword beside `$ref`,
   because the rule for new keywords says to take the standard's, and a
   dependency already is the other system.
3. **The bypassed check is the gate's question, and a route both
   people and clients call names them in its `callers`**, rather than
   0.3 gaining `callers: all`; what the owners of such a system answer to
   the gate's question decides whether it is a design or a defect, and
   where it is a design the answer is the client in that operation's
   `callers`, the explicit subset the caller context designs
   (`docs/caller-context.md`, ADR-110). The caller's context as a whole
   is designed on its own, outside this plan.
4. **The readers are inside the binary**: build item 8. The JavaScript
   reader is built on the TypeScript compiler's own Go source copied into
   the repository at a pinned tag (decided 2026-10-14, after TypeScript 7
   shipped its compiler in Go); the Swift and Dart readers are carried by
   the binary and built in a per-user cache.
5. **A standard workflow format gets a reader (a plug-in); a custom
   one gets a workflow table its project's printer writes.** The
   estate's workflow definitions are custom JSON, so build item 9 is
   the table.
6. **A reader may write an implementation fragment** (the database
   reader's indexes under `mappings`, with `--implementation-out`),
   since the mapping is as mechanical as the entity and the procedure
   of `docs/from-sources.md` section 3.4 writes it from the code anyway.
7. **`ownedBy` on a requirement in the requirements stage**, rather
   than a `status` value for a requirement held elsewhere: one word for
   one meaning, and `status` keeps 29148's life cycle.
