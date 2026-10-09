# Targets

What every SpecArch target must do, document or code, and the pattern each
one follows. `specarch document <target>` writes a document; `specarch
generate <target>` writes code or data, through a plug-in.
None of the patterns is new. Each was found running in production, in a tool
that already does one piece of the job well: sqlc for queries, protoc for
service contracts, the OpenAPI code generators that offer a strict server
mode, migration tools that only ever add a file. SpecArch copies those
patterns rather than inventing its own, and where a standard tool exists for
a step, the emitter stops at that tool's input and lets it do the rest.

## Rules for every emitter

1. One command, one folder. `specarch document <target> <folders>` and
   `specarch generate <target> <folders>` write into a folder the target
   owns and nothing outside it: the folder the implementation file names
   under `targets`, or `--out`. Every generated file starts with a header
   naming the root file, its `info.version` and the meta-model version it
   came from.
2. Generated output is never edited by hand. A change goes into the spec and
   the output is regenerated. With `--check` the output is made in memory
   and compared with the disk, and the run fails when it differs. In CI
   that is the third gate in `docs/sync-gates.md`.
3. Hand-written files are never touched. The one exception is a Markdown
   document, which is rewritten only between its `specarch:generate`
   markers.
4. What the target cannot express fails generation. A check constraint the
   database cannot evaluate, a decimal the language has no type for, a page
   kind the component library does not render: each is an error naming the
   object. There is no warning-only mode, because an omission that is only
   warned about is the same as a silent one by the next release.
5. One reviewed input becomes one typed output, and the input is the thing
   under review. This is the sqlc rule: the query is the spec, the function
   is derived, and nobody reviews the function. For SpecArch the YAML is the
   input and the review happens there.

## Plug-ins

A code target that `specarch` does not build in is produced by an
executable found on PATH, the pattern of `protoc-gen-*`, `git-*` and
`kubectl-*`, with protoc's protocol: the plug-in never touches the disk.
For each implementation file that names the target, the executable is
`specarch-gen-<target>-<stack>` when there is one, the stack being the
file's language in lower case (`specarch-gen-tests-go`), and
`specarch-gen-<target>` otherwise. One target can then have a plug-in per
stack and one that serves every stack; files that find the same plug-in
and name the same output folder run it together, each run writing into the
folder its files name, so two front ends of one stack are two
implementation files, each with its own output. `specarch` validates the specification, refuses
while a must or should question blocks a section the target reads (the
sections the implementation file names under `targets.<target>.reads`, or
every section when it names none) and refuses without an approval record
of the files as they are unless `--unapproved` is given (`docs/refinement.md`),
then runs the plug-in once per specification, and writes on its standard
input one JSON object:

| Key | Holds |
|---|---|
| `specarch` | the meta-model version |
| `target` | the target name |
| `root` | the root file's path |
| `specification` | the merged, validated specification as plain values |
| `implementations` | one object per implementation file: `file`, `content` (the file as plain values), `settings` (the target's settings from it, when any), and `idioms`: each idiom the file uses, with its `name`, `version`, how it applies (`as`: shipped, overridden, project or excluded), the `content` of the idiom that applies and the project's `override`, so a plug-in renders through an override without reading the disk |
| `existing` | the text files already in the output folder, each a `path` in it and its `content`, so a plug-in that adds files knows what is there |
| `output` | the folder the files are for |

The plug-in answers on its standard output with one JSON object: `files`,
each a `path` relative to the output folder and its `content`, and
`diagnostics`, each with the validator's fields (`file`, `line`,
`severity`, `path`, `rule`, `message`), and exits 0. `specarch` prints the
diagnostics, refuses a path outside the output folder, and writes or
compares the files itself, so `--check` and rule 1 hold for every plug-in
without each one implementing them. A plug-in that exits with another
status, or answers with anything else, fails the run with status 2; its
standard error is passed through. The request and answer structs are in
`cmd/specarch/generate.go`.

A plug-in that builds code or data leaves out every element another
stakeholder owns: one whose mapping in an implementation file of the
request names a stakeholder under `ownedBy`, and everything under its
pointer (`docs/conventions.md`, ADR-046). It still writes what refers to
an owned element, such as a foreign key to an owned table. The shipped
plug-ins read the marks through `internal/ownership`. A plug-in that
writes tests keeps owned elements, since a test checks the running system
rather than building it.

## Markdown technical specification

`specarch document techspec` writes `techspec.md` for a specification: the
arc42 chapters of `docs/conventions.md`, each only when the specification
has something for it, with the generated Mermaid diagrams and tables.
Chapter 2 comes from the constraints and assumptions, chapter 7 from the
deployment and commissioning stages and then from each implementation file
(stack, libraries, layout, mappings, bindings, targets, tasks, testing,
deployments and the implementation decisions), chapter 12 from the
glossary, and chapter 13 from the needs and requirements with the
traceability matrix of what satisfies and what verifies each requirement.

## The other documents

`specarch document requirements` writes `requirements.md`: purpose and
scope, the stakeholders, the needs with the requirements that refine them,
each requirement with its attributes and acceptance criteria, then the
constraints, assumptions and glossary. `testplan` writes `testplan.md`: the
count of golden and red tests, the levels, how each implementation's suites
run them, and every design test as a test case under its subject.
`traceability` writes `traceability.md`: needs to requirements,
requirements to what satisfies and verifies them, and the gaps.
`deployment` writes `deployment.md`: the environments and the path a
release takes, the settings, each installation of each implementation with
its servers and setting values, a secret only named, then release, rollback
and migrations. `commissioning` writes `commissioning.md`: the checks by
environment in the order a release reaches them, a Result column for every
step, and the sign-off sheet. `questions` writes `questions.md`: the open
questions by stage with what each blocks and who decides, and a table of
the outputs, each ready, a draft or waiting; `specarch gaps` prints the
same text and exits 1 while a must or should question is open. `manual`
and `operations` wait until the specification holds what they need.

In every document, an element's `why` is a paragraph that starts with
**Insight:** and each of its citations one that starts with **Note:**,
under the element's heading, or after the table when the element is a row,
labelled with the row's name. Before them, an element that says how it is
known gets one line, **Origin:**, and an element an open question blocks
gets one paragraph per question, **Open question Q-12 (must, decision):**.
A document whose sections a must or should question blocks starts with a
**Draft:** notice under its summary. A document that cites sources ends
with a table of them.

The techspec target also rewrites the generated diagrams and tables between the
markers of the hand-written `specarch.md` beside the root file and leaves
every other line alone. A document with no markers gets nothing; a marker for an
object that does not exist is an error, not an empty block, so a deleted
entity cannot leave a stale diagram behind.

## OpenAPI document and the server interface

`specarch-gen-openapi` writes `openapi.yaml`, an OpenAPI 3.1 document, field
for field, since the keywords are OpenAPI's own; this is the standard
dialect. An entity or an enum is a schema under `components`, and an
audited entity's schema carries its four audit fields as read-only.
SpecArch's own field and operation keywords become `x-specarch-`
extensions (`permission`, `limits`, `emits`, `satisfies`, `precision`,
`scale`, `sensitivity`, `atRest`, `lookup`); the rationale and the test
hints are left out. Permissions become the security scheme the target's
settings name under `securityScheme` (its `name` and the OpenAPI scheme
object), required on every operation that is not public, plus the
`x-specarch-permission` extension; without the setting the document has
no scheme. A response that names a problem type answers
`application/problem+json` with RFC 9457's problem schema and names the
type in `x-specarch-problem`; the catalogue is `x-specarch-problems`. A
list takes the names of the paginated-list idiom that applies: query
parameters on a GET, properties of an inline request body on another
method, and the answer wrapped in the idiom's envelope. A view is a
read-only schema of its own, marked `x-specarch-view` with the entity it
reads from: that entity's fields and the ones the view adds, each added
one read-only. A path has the type of the field it ends in, with null
allowed when a relation on the way may have no record or the field is
not required, and a count is a
64-bit integer of at least 0. A list over a view filters and sorts by the
view's fields.
When the specification says `info.wireNames: snake_case`, every property
name the document writes goes on the wire in snake_case (`loanedOn` is
`loaned_on`): the entities', views' and inline objects', the required
lists, the audit fields, the fields a list filters and sorts by, and the
idiom's paging parameters and envelope fields. A parameter keeps the name
the specification gives it (ADR-062).

With `dialect: dxlib` on the target, the document is the one dxlib's OpenAPI
reader binds. Every operation is a POST at `/<operationId>` with all of its
parameters in one JSON body, dxlib's own command convention, since dxlib
routes by the URI alone. A field carries its dxlib type in `x-dxlib-type`
and keeps only the constraints dxlib's validator applies; the others, a
`maxLength`, a `pattern`, a `maximum`, a format dxlib does not know, are
listed in `x-specarch-unenforced` on the field, for the go-dxlib handler to
check. A field that may be null takes dxlib's nullable type where its base
has one and is not required, with a warning where the design requires it,
because dxlib reads a null and a left-out parameter as one case, not given.
Permissions are `x-dxlib-privileges`, no security scheme is written,
a refusal is dxlib's error body named by its problem type, and a list uses
the dxlib names and envelope of the paginated-list idiom. Two limits are
dxlib's to lift: its validator does not apply JSON Schema's bounds, and its
routes carry no method.

## Go for dxlib

`specarch-gen-go-dxlib` writes `specarch_dxlib.go` for the `go-dxlib`
target, in the package its `package` setting names (`service` when none)
for the database its `databaseNameId` setting names, which it needs. The
file holds a dxlib table per entity, a DXTable when the entity is audited
or softly deleted and a DXRawTable otherwise, with the search, order and
filter fields its lists allow. When an entity is listed through a view,
the table pages through that view as its list view, named as
`specarch-gen-sql` names it; dxlib then reads the table by key and counts
through the view too, which works because a view carries every field of
its entity. An entity listed through two views, or both directly and
through a view, is refused. The file also
holds `Register`, binding each handler to its `operationId`, and per
operation a request struct with a `Has` flag per
field, read through dxlib's typed getters, the field's design default
taken when the value is not given (dxlib reads a null and a left-out
parameter alike, so `Has` is false for both), a check of every constraint the
dxlib dialect lists as unenforced, and the handler. A list checks the page
size and runs the table's paging list, a create inserts the given fields
with a new identifier and the audit fields and answers the stored row with
201, and a read by identifier answers the row or the entity's not-found
refusal. Any other operation calls `body<Operation>`, and each job that
repeats at an interval becomes a task in `RegisterTasks` calling
`job<Name>`; the service writes both, and the compiler names what is
missing. Permissions, roles and the menu are `Privileges`, `Roles` and
`MenuItems`, data for the service to seed, `public` left out. Wire names
are snake_case, as in the dxlib dialect, because dxlib's standard
operations take a parameter's name as its column's.

The file holds no data model, since `specarch-gen-sql` writes the schema.
Encryption keys, scheduled and consuming jobs, and the seeding stay the
service's, and the generator warns where a design asks for them.

## Server code on other stacks

For a stack without a SpecArch generator, a standard OpenAPI code generator for the
stack turns the document into a typed server interface with request and
response types, and hand-written handlers implement that interface. The
compiler then refuses a handler whose signature does not match the spec. Use
the generator's strict mode where it has one: decoding the request and typing
the response move into generated code, so a handler cannot return a status
or a body the spec does not list.

The contract is fixed before either side is built. An operation may be
answered with 501 Not Implemented while its contract is final; the client is
built against the contract in the meantime, and the route-table gate sees the
stub as a served route. A list of stubs is a report, not a failure.

Where services talk to each other through an interface definition language
such as protobuf, the same rule applies: the interface file is the contract,
reviewed and fixed first, and the rest is derived. An emitter for such a
format is generator 5 on the roadmap, written when a real project needs it.

## SQL migrations

`specarch-gen-sql` writes the migrations in the `sql` target's dialect,
PostgreSQL, SQL Server, Oracle or MariaDB, each column through the
type-rendering idiom's rows for that dialect (`docs/idioms.md`): the first
row that matches the field gives its type, and the row's check becomes a
check constraint. Names follow `docs/conventions.md`. The audit,
soft-delete and encrypted-column idioms give their columns, an encrypted
field looked up by hash gains the hash column its keys use, and a key or
unique text column needs a `maxLength` of at most 255. The first migration
is `0001_expand.sql`, with `snapshot.yaml` beside it; a run on an
unchanged schema writes only the snapshot again, so `generate --check`
passes until the schema changes. A change is written as the next
`NNNN_expand.sql`, and what can lose data as the next `NNNN_contract.sql`,
which the `sql` target's settings must allow with `destructive: true`;
a change the differ cannot tell from a rewrite (a new type, a new key, a
new default, a new required column without a default) is refused with
what to do by hand.

Each view becomes a SQL view, created after the tables and their foreign
keys. It lists its entity's columns by name, joins each relation a path
follows with a `LEFT JOIN`, so a record with no related record is still
listed, and counts a relation to many in a subquery that leaves out softly
deleted records (`COUNT_BIG` on SQL Server, whose `COUNT` is 32 bits). On
SQL Server the statement runs through `EXEC`, because `CREATE VIEW` must
be the first statement of a batch. A view's column list is fixed when it
is created, and PostgreSQL refuses to change the type of a column a view
reads, so a migration that changes a view, an entity it reads or an enum
of a field it reads drops that view first and creates it again last, in
the expand migration and again around a contract one. A view holds no
rows, so this never needs `destructive: true`; on MariaDB, Oracle, and SQL
Server outside a transaction, the view is missing while the migration
runs, and a list through it fails until the migration ends. An added
field named like an audit, deleted or hash column, a path that ends in a
field only written, and a many-to-many count whose join entity has more
than one relation to either side are refused.

Migrations are new files only. The emitter keeps a snapshot of the spec as it
stood after the last generated migration, diffs the current spec against it,
writes one new numbered forward migration, and updates the snapshot in the
same run. The snapshot is committed with the migration. A migration that has
been applied anywhere is never regenerated or edited: regenerating it would
change history that a database has already recorded.

An entity or a view another stakeholder owns gets no table or view and no
statement in any migration; a foreign key to it is still written. The
snapshot keeps its shape and lists its pointer under `owned`, so marking a
table that this project created as owned hands it over without a drop,
and taking one back starts from the shape the specification last gave it.

A destructive step (dropping a column or a table, narrowing a type) is
emitted only when asked for, by the target setting `destructive: true`, and
always as a file of its own. That
keeps the expand-contract pattern honest for a column that live code reads:
add the new column in one release, backfill it, switch the code, and remove
the old column in a later release, each in its own migration. The emitter
cannot tell a rename from a drop and an add, so a rename is written as those
two releases.

Constraints become real database constraints. A `check` expression is
translated into the target's dialect; one the target cannot express fails
generation (rule 4). Names are derived with the rules in
`docs/conventions.md`, so the database gate can derive them the same way.

## UI page definitions

Where the target project already renders lists, forms and views from a schema
object that its component library reads, the emitter writes that schema
object and nothing else. The screen then looks like every other screen in
the project, because the same library draws it. Only when no such layer
exists does the emitter write components, and then it uses the project's own
components in the project's own style.

An emitter that writes whole pages in a state-management style the project
does not use is dead on arrival; one such generator was found in a codebase,
unused, next to the screens people had written by hand instead. The first
emitter for a component library is therefore written against one hand-built
screen in a real project and has to reproduce it before it is accepted.

### Plain JavaScript for the web

`specarch-gen-ui` writes, for a `ui` target of platform `web` and
framework `plain-javascript`, each list page as `<page>.html` and
`<page>.js`, every event of the screens once in `events.js`, and
`theme.css`. Nothing needs a package, a bundler or a build step: the pages
load native ES modules, and no third-party file is written. A page reads
its list a page at a time by the paginated-list idiom's names, with only
the answer to its latest request counting and a page past the last
falling back to the last, filters by a query parameter of its source
operation or, through the idiom's `filter` name, a field its `listOf`
makes filterable (any other filter is refused), hides on a compact screen, below 600 CSS pixels, the columns
`compactColumns` leaves out, and runs each operation action on a row,
taking a path parameter from the row's primary key, after its confirmation
and with its message, its button disabled while it runs; each row's
buttons are named with the row's first column, and focus stays on the
same button when the list is read again. A list whose operation's path
takes a parameter, or whose operation does not page, is refused; an
action whose operation takes a request body is reported and left out. A
service whose OpenAPI target is in the dxlib dialect is refused, since
its lists are not the standard dialect's. Its states and the messages of its events show in a
status line announced without moving focus; a refusal shows the failed
state of its problem type, matched by the operation's response status. A
page without states shows the generator's own sentences in English.

Every event is an UPPERCASE constant with its payload, declared once in
`events.js`, and each part of a page subscribes where it is created; set
`DEBUG` there to log every event. `theme.css` holds the tokens as custom
properties, the dark mode under `prefers-color-scheme`, and the rules
every screen shares, which use the tokens the target's `settings.tokens`
names for each part: `text`, `background`, `accent`, `danger`, `border`,
`space` and `font`. `settings.language` is the pages' `lang`, and is
required. A token's name must be letters, digits, `-` and `_`, and two
tokens may not make one custom property; a font family other than CSS's
generic ones is written as a string. Two events of the screens with one
name, a page's and an operation's, are refused.

Forms, views, navigation (`onSelect`, actions of kind navigate, an
action's `then.navigate`) and modes other than dark are reported and left
out of this version. The screen it is checked against is the library
lending example's loans list, written by hand before the generator; the
first real plain JavaScript project repeats the check against its own
first screen (ADR-040).

### TypeScript on Next.js and Carbon

`specarch-gen-ui-typescript` writes, for a `ui` target of platform `web`
and framework `nextjs-carbon` in an implementation file in TypeScript,
each page as `page.schema.ts` and `page.tsx` under `app/` at the
page's route (`{param}` becomes `[param]`), the components that read the
schemas once under `screens/`, and every text once in `strings.ts`, in the
language `settings.language` names, which is required. The schema file is
data only: one object literal typed with `satisfies`, whose keys are the
names of the `ui-components` idiom (`docs/idioms.md`), a project's
override first, laid out as prettier lays out an object literal at a width
of 140. `page.tsx` hands the component the schema, the texts it names and
the routes its events lead to. On a task page, a field is a property of
the request body of type string, drawn by the text, email or password field part by its
format and checked by its keywords before the request is sent; a password
shows its rules under it. A check across fields is written as a rule in
the data, of names, constants, comparisons, `&&`, `||`, `!` and `size`; any
other operator or function is refused at the check. A refusal shows under
the field its failed state names, or above the form; each success leads to
its page, its message announced there. A page reads `returnTo` from its
query and hands it on to another task page, and an event to any other
page goes to it instead, when the browser reads it as the same origin.
A default failed state is the message for any refusal the page names
none for. Fields in sections, actions, `enabledBy`, a permission other
than public, and a success the operation answers with no event are
refused. The generator writes no package, lock file or configuration, and
refuses to write over a file in its output folder it did not write.

Each list page is a schema and a `page.tsx` too, behind the guard. The
service pages, sorts, searches and filters, through the paginated-list
idiom's names on the wire: a column sorts when `listOf` makes its field
sortable, a search box shows when `listOf` names searchable fields, and a
filter is the operation's query parameter or, through the idiom's
`filter` name, a field `listOf` makes filterable, an enum drawn as a list
of its values. The list has a column picker, a refresh that keeps its
page, and keeps `compactColumns` below Carbon's medium breakpoint. An
action of kind navigate is a button of the toolbar; an action of kind
operation is in each row's menu while its `when` holds for the row and its
permission is held, and runs after its confirmation, sending the reason
it names as the only property of its body; `onSelect` opens its page from
the row's first column.

`application.ts` holds the menu and how the session is read:
`settings.session` names the operation that answers who is signed in, the
boolean and the list of permissions in its answer, and the page that
signs in. The guard and the menu ask one function whether a page opens to
the person, so a menu entry shows exactly when its page opens; the guard
sends someone signed out to sign in with `returnTo`, shows anyone else the
refusal, and never makes a refused page, which so calls no operation.
Each list, form and view that needs a permission gets `tests/<page>.test.ts`, and the
menu `tests/menu.test.ts`, derived tests node's test runner runs. The
side navigation draws one level of groups, so a group inside a menu is an
error at the entry, and so is an entry that opens a page whose route
takes a parameter. An entry whose page this version does not write is
left out with a warning at the entry, and comes back when its page is
written; an entry for a page another stakeholder owns stays. A page, a
menu or a menu entry marked `ownedBy` is left out. A part of the
idiom that names `import` draws through that package, so a project's
override renders its lists through its own library (`docs/idioms.md`).

Each form and each view is a schema and a `page.tsx` behind the guard,
with its derived test, the route's parameters handed to the page. A form
without `source` creates a record; one with it loads the record first,
from the operation `source` names, its path filled from the route. Its
fields come in its sections, each under its title, on one page, as tabs or
as steps, as `settings.layouts` says per form. A field is drawn by the part
of its property's type and format: text, a text area for a string longer
than 255 characters or of no maximum, email, password, number, date, an
enum's values, a check box; a field the page picks is a lookup that reads
its source a page at a time, searched through the service when `listOf`
names searchable fields and otherwise among the one page read, with a
warning at the picker. A field is required when the request body requires
it, is checked by its keywords before the request is sent, and is typed a
second time when the page lists it under `enteredTwice`. `readOnly`,
`readOnlyWhen`, `hiddenWhen` and the checks are rules over the record, as
on a task page, the form's values typed as the service has them and an
empty field null; a hidden field is neither checked nor sent, and a rule
that orders a decimal is refused, since the browser cannot compare one
exactly. The request sends the fields its body takes, and an idempotency
key in the header the operation names: kept for the retry of a request
that got no answer, and chosen anew after an answer, since a key used
again is answered as its first request was. A field
`settings.hooks` names for a form starts with the value of the hook of its
name: `page.client.tsx`, which the generator writes, imports it from
`page.hooks.ts` beside the page, which the project writes, so a missing
hook fails `tsc`. A view shows its sections read-only, a field hidden while
its condition holds, and offers the actions that open a page to whoever
holds their permission. A required property of the request body the form
does not show, a field the body does not take that is not read-only, and a
hook or a layout for a page that is not a form or a field it does not show
are refused.

A form's child rows are sent as the array of its request body named for
the relation, each row with the fields the array's items take; a field
the items do not take is shown on the loaded rows only. A form that loads
its record reads the loaded rows from the property of the view its source
answers that adds the relation's rows (`rows`), shows them read-only and
sends only the rows added, so its loaded rows must be locked. Added rows
are drawn inline, checked by their keywords, at least the body's
`minItems` and, with the loaded ones, at most the maximum. A view's action
that runs an operation is offered while its `when` holds for the record,
asks the list's confirmation with the reason it requires, and reads the
record again after a success; an action that leads to another page once
it succeeds is refused. A form that submits to a workflow's trigger acts
on the 202 answer, its event the page's pending state, and an inbox is a
list whose row actions are the approver's operations. Rows in a dialog
are not written, and a page does not follow an approval to its end, since
that end is a channel message no browser reads (ADR-071).

The screens it is checked against are
the library lending example's sign-in and second-factor screens, written
by hand before the generator (ADR-067); its lists are built on plain
Carbon and through the stub of a fictional library (ADR-068), its forms
and views on both (ADR-069), and its child rows, reasons and approvals on
both (ADR-071).

The screens call the service at `NEXT_PUBLIC_API_URL`, or, with
`settings.server`, the application's own server routes: a `route.ts` per
path a page calls, under the path `settings.server.routes` names, with a
handler per method called and none for an operation marked `ownedBy`. A
handler forwards the method, the body and the accept, content-type and
idempotency headers to the service at the environment variable
`settings.server.service` names, read on the server only, with the value
of the cookie `settings.server.token` names in the header it names after
its scheme and no cookie of the browser; it answers the service's status,
body, type, location and cookies, so the cookie a service sets when a
session opens lands on the application's origin. A GET named under
`settings.server.pages`, whose answer is an array and which has no
`listOf`, is read whole and answered a page at a time in the paginated-list
idiom's envelope, at the page size and maximum given there; the list that
reads it offers no sort, search or filter but its query parameters. A
service variable starting `NEXT_PUBLIC_`, which Next.js sends the browser,
a page under the routes' path, and a paged operation that pages itself or
answers no array are refused. `tests/server.test.ts` runs the forwarding
and the paging against a stub of fetch.

`settings.translations` lists the languages besides `settings.language`,
each read from `strings.<language>.json` in the output folder, a JSON
object of string keys and texts the project writes. A key the screens use
with no entry, an entry with no text and one that drops a placeholder such
as `{count}` are errors, and an entry no screen uses is a warning.
`strings.ts` holds every language, typed so each translation has every
key; a page reads the language from the cookie the `LanguageChoice`
component sets for a year, through `chosenLanguage` in `language.ts`, and
the specification's language without one.

`theme.scss` lays the theme over Carbon's: its light theme in `:root` and
its dark theme under `prefers-color-scheme: dark`, each merged with the
colours `settings.tokens` maps from a part (text, background, accent,
link, danger, border) to a token of the theme, the dark mode's value where
it gives one. Which Carbon token takes each part is the `design-tokens`
idiom's (`docs/idioms.md`); a part no Carbon token takes, a token that is
not a colour and a mode other than dark are reported, and the application's
own styles use the file after Carbon's (ADR-074).

## Tests from the specification

The specification says what is tested; the implementation file says with
what. One test per worked example and one per design test, in the
framework the implementation file's `testing` names, each named after its
example or test.

A worked example's test calls the hand-written algorithm body with the
example's `inputs` and asserts the `expected` value, with every value in its
declared type: decimals as decimals, never as floating point. The validator
has already checked that the formula gives `expected` on those inputs, so a
generated test that fails points at the body, not at the spec.

A design test becomes a test with three marked steps, its `given`, `when`
and `then`, and a body the implementation fills in where the sentences
cannot be turned into code.

For Go, `specarch-gen-tests-go` writes one file, `specarch_design_test.go`,
into the folder of the `tests` target. The file declares a `Harness`
interface and calls `newHarness(t)`, which the project writes in a file of
its own beside it: how a caller signs in, how a record is inserted and read
back through an entity's mapping, how an operation is called (the path as
the specification writes it, with its parameters filled from the input),
a command run, a page opened, and an algorithm's mapped function computed.
A test whose design gives a fixture, an input and an expected outcome is
complete; one without a call calls `body<TestName>(t, h)`, which the
project writes too, so the package does not compile until every such body
exists. Values carry their type in the expression language, and decimals
are compared as decimals. The package name is `specarchtests`, or the
`package` the target's `settings` give, which must be the package of the
other files in that folder. Its subject and scenario go into the test's
name, so a failing red test says which refusal broke. A design test marked
`notApplicable` becomes no test; its reason is printed in the generated
file where the test would be.

Swift and Dart get the same tests, calls and checks from the same reading
of the specification, and differ only in the language; a Flutter app's
language is Dart, so `specarch-gen-tests-dart` serves it. The
implementation file's `testing.framework` picks what the tests are
written for, and a framework the plug-in does not write for is refused by
name with nothing written.

- `specarch-gen-tests-swift` writes `SpecArchDesignTests.swift` for Swift
  Testing, the only framework it writes for. The values, the `Harness`
  protocol and the checks are in a caseless `enum SpecArch`, so they do
  not collide with the test target's own types. The project writes
  `func makeHarness() async throws -> any SpecArch.Harness` and, for each
  test the design gives no call for,
  `func body<TestName>(_ h: any SpecArch.Harness) async throws`. A check
  records an issue at the line that called it, and the test goes on.
- `specarch-gen-tests-dart` writes `specarch_design.dart`, the values,
  the `Harness` interface and the checks, and `specarch_design_test.dart`,
  the tests, for `package:test` or `flutter_test` (`package:test` when the
  file names none). The project writes `specarch_harness.dart` beside
  them, importing `specarch_design.dart`, with
  `Future<Harness> newHarness()` and
  `Future<void> body<TestName>(Harness h)` for each test the design gives
  no call for. A check fails the test where it stands.

The harness methods are asynchronous and may throw on both stacks, since
a harness talks to a server, a database or a widget tree; one that does
not need to can answer at once. Decimals and numbers are compared exactly,
whatever their length, with the standard library alone: the whole text
must be a number, and both are brought to one form, digits and a power of
ten, so the generated tests bring the project no dependency. When the
implementation files reached by one plug-in name frameworks, each must be
one it writes for. ADR-041 has the reasons.

## Guarded operational scripts

A data change run against a live system (a new role and its permissions, a
backfill, a one-off correction) is safer emitted than typed. The emitted
script has preconditions that raise when the system is not in the expected
state, a dry run as the default, postconditions that check exact row counts,
and no effect on a second run.

The `guard` on an operation or a command (`docs/conventions.md`, Guards)
is what the script is emitted from: its `precondition` becomes the check
that raises, its `recordsChanged` the exact count the postcondition
asserts. The emitter is a later item.

## Other formats

Another DSL or configuration format is emitted on request, limited to what
that format can execute. A concept it cannot represent is reported by name
(rule 4). A format that can only hold part of the spec gets a partial
document that says so in its header.

## Adding a target

A new target is written against a real project, not against the example.
It is accepted when its output is what that project would have written by
hand, its `--check` form runs in that project's CI, and the matching gate in
`docs/sync-gates.md` is on. Code targets are plug-ins, added one stack at a
time, in the order the real projects on the roadmap need them; document
targets are built into `specarch`, one per stage of the life cycle.
