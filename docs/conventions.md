# Conventions

How a SpecArch specification is laid out on disk, how the YAML is organised,
what the Markdown sections are, and which diagrams are generated.

## The tree

A project keeps its specification in a `spec/` folder at the repository root
(or wherever its own rules say). The specification is that folder: its root
file `specarch.yaml`, and one folder per life-cycle stage it keeps.

    spec/
      specarch.yaml                  the root: specarch, info, stages, sources
      specarch.md                    the hand-written explanation, with generated regions
      requirements/                  stakeholders, needs, requirements, glossary, assumptions, constraints
      design/                        enums, entities, permissions, roles, paths, commands, channels, pages, algorithms, decisions
      implementation/<stack>/        <name>.<stack>.specarch-implementation.yaml, and its .md when it needs one
      tests/<name>/                  test.yaml, and the scenario's own input and expected-output files
      deployment/                    environments, configuration, release, rollback, migrations, handover
      commissioning/                 checks, signoff
      operation/                     monitors, backup, restore
      <stage>/questions.yaml         the open questions about that stage, in any stage folder

The rules, each of which the validator checks under the rule `layout`:

- The root file lists the stages it keeps under `stages`, in life-cycle
  order. A listed stage has a folder of that name; a folder that is not a
  listed stage is refused; a listed stage without a folder is refused.
- A stage that is not listed has no folder. Its sections, if it has any,
  are written in the root file itself; an implementation file then sits
  beside the root file. So a tiny specification is one file, and a project
  adds a folder when a stage grows.
- A file under a stage folder is a mapping whose keys are sections of that
  stage, each holding named objects: `entities: { Loan: ... }`. A file may
  hold one object or a few. A sub-folder named after a section holds only
  that section. The convention, used in `spec/` and the example, is one
  sub-folder per section and one object per file, named after the object
  in kebab-case (`design/entities/loan.yaml`, `design/decisions/ADR-001.yaml`),
  with the small sections (`permissions`, `roles`) in one file each. The
  single-object sections (`release`, `rollback`, `handover`, `signoff`,
  `backup`, `restore`) are one file each, `deployment/release.yaml`.
- Each test is a folder `tests/<name>/` holding `test.yaml`, the test
  itself, beside any input and expected-output files the scenario needs;
  the validator reads nothing in a test folder but `test.yaml`. A file
  directly under `tests/` is refused.
- An implementation file lives under `implementation/<stack>/`, and the
  folder's name is the stack in the file's name.
- Files end in `.yaml`; `.md` and other files in a stage folder are left
  alone. Folders starting with a dot are skipped.

The validator reads the tree as one document, in the shape the schema
describes: the root file's keys, then every section merged from its files.
A name is defined once within its section across the whole tree; a second
definition is refused with both files named. Because the document is one,
every reference is the plain key it would be in one file: `target: Loan`,
`$ref: "#/entities/Loan"`, `satisfies: [LIB-3]`, `source: iso-29148`. No
reference ever names a file, so objects move between files without
breaking one. A diagnostic names the file and line of the node and the
YAML path in the merged document, which is also the path an implementation
file's pointers use.

The first line of the root file is the editor hint:

    # yaml-language-server: $schema=https://raw.githubusercontent.com/SpecArch/specarch/v0.7.5/schema/specarch-design-0.3.schema.json

For an implementation file:

    # yaml-language-server: $schema=https://raw.githubusercontent.com/SpecArch/specarch/v0.7.5/schema/specarch-implementation-0.1.schema.json

A hint names the newest release tag that serves its schema. A release
whose schemas changed moves every hint, and the schema line `extract`
writes, to its own tag before tagging; a release whose schemas did not
change leaves them at the tag they name (`docs/deployment.md`, Release).

The design schema describes the merged document, so a file under a stage
folder, which holds only some of its sections, has a schema of its own:

| File | Its schema |
|---|---|
| a file under `requirements/`, `design/`, `deployment/`, `commissioning/` or `operation/` | `specarch-fragment-<stage>-0.3.schema.json`, that stage's sections and `questions` |
| `tests/<name>/test.yaml` | `specarch-fragment-test-0.3.schema.json`, one test |
| a file directly under `tests/` or `implementation/` | `specarch-fragment-questions-0.3.schema.json`, only `questions` |

A fragment's hint names the same commit or tag as the root file's, here
for a file under `design/entities/`:

    # yaml-language-server: $schema=https://raw.githubusercontent.com/SpecArch/specarch/<commit or tag>/schema/specarch-fragment-design-0.3.schema.json

Each is written from the design schema by `tools/fragment-schemas`, which
copies the stage's sections and the definitions they reach, so a keyword
is defined once; `go test ./...` fails when the design schema changes and
they were not written again (`go run ./tools/fragment-schemas`). One file
alone cannot show what spans files: a reference to an object in another
file, a name defined twice, a sub-folder holding a second section, and a
required key an open must question blocks, which the editor reports as
missing and `specarch validate` excuses while the question is open. The
editor checks each file as it is written; `specarch validate` checks the
whole tree.

The specifications in this repository name the schemas by a path relative
to the file (`../../../schema/specarch-fragment-design-0.3.schema.json`),
because they are written against the schemas beside them rather than a
released tag; `go run ./tools/fragment-schemas` writes those lines too.

Pin the schema version a project is written against. A project moves to a
new meta-model version on purpose, in its own change.

## Design and implementation

A specification has two sides, and every fact belongs in exactly one of
them.

The **SpecArch Definition** (SDF) is the design side: the root file and
every stage folder but `implementation/`. It is neutral about language,
compiler and framework: stakeholders, needs and requirements; entities,
enums, operations, commands, events, pages, permissions and roles,
algorithms with formula, worked examples and pseudocode; tests; environments
and who runs each, settings, release, rollback and migration steps, and the
handover to a customer who runs an environment; commissioning checks and
sign-off; monitors of the live system, backup and restore; and the
decisions that hold whatever the stack.

A **SpecArch Implementation File** (SIF, `<name>.<stack>.specarch-implementation.yaml`,
root key `specarchImplementation`) is one stack's implementation of one
specification. It names the root file and the `info.version` it was written
against under `implements`, and holds everything that depends on the stack
and nothing else: the language and toolchain with versions (`stack`), every
library with its version and licence (`libraries`), the package layout and
the design objects each package implements with the packages it depends on
(`layout`), how each design
object maps onto the stack (`mappings`, each of which may carry `why`,
`cites` and `origin` like any element, and `ownedBy`, below), the framework per interface kind
(`bindings`), the document and code targets with their folders and settings
(`targets`), build, test and CI commands (`tasks`), the test suites with
their levels (`testing`), the shipped idioms it excludes or overrides, each
with the reason (`idioms`), real servers, hosts, ports and the values of
non-secret settings per environment, with how a release is built, how it
is installed and what the hosts must provide (`deployments`), and
implementation decisions (`decisions`). The idioms SpecArch ships, under `idioms/`, apply
to every implementation file by default; an override or a project's own
idiom is a file in the `idioms/` folder beside the implementation file,
checked against `schema/specarch-idiom-0.1.schema.json`. `docs/idioms.md`
is the design, and `specarch idioms` lists what each file uses. Its schema,
`schema/specarch-implementation-0.1.schema.json`, has no keyword for an
entity, an operation or any other design object, so an implementation file
cannot add or change design. One specification can have several
implementation files, one per stack; the `<stack>` part of the name tells
them apart (`library-lending.go.specarch-implementation.yaml`).

A layout entry's `dependsOn`, CycloneDX's word, lists the other layout
keys the package calls, imports or links against; the technical
specification draws them as the implementation's component diagram, and
the marker `components <stack>` draws it in a hand-written `specarch.md`.
A name that is no key of the layout, or the entry's own key, is an error
at the entry (`depends_on`); a cycle is not, since Go forbids one and
other stacks allow it. An entry without `dependsOn` says nothing of what
it depends on, and the diagram draws no arrow from it. `extract go
--implementation <file> --implementation-out <new file>` writes, for each
layout entry whose Go files it reads, the packages of the same module
they import, test files left out (ADR-133).

A deployment's server may say what it is and what runs on it, for the
deployment diagram (ADR-136):

    servers:
      - { name: api, url: "https://library.example", runs: { packages: [internal/api] } }
      - { name: db, kind: database, url: "postgresql://db.internal:5432", runs: { targets: [sql] } }
      - { name: bus, dependency: eventBus, url: "amqps://bus.internal:5671" }

`name` is kebab-case and unique within the deployment. `kind` is one of
`service` (a host that runs this implementation's packages; left out, a
server is one), `database`, `messageBroker`, `proxy` and `fileStore`, the
dependency's words where the thing is the same. `runs` lists layout keys
under `packages` and target keys under `targets`. `dependency` names the
design's dependency the server is in this environment; it is then drawn
as that dependency and says no `kind` or `runs` of its own. Each of these
is checked at the entry under the rule `server`: a name used twice, a
package or target the file lacks, packages on a server not of kind
`service`, a `kind` or `runs` beside `dependency`, and an undeclared
dependency. The technical specification and the deployment guide draw a
deployment once one of its servers uses a key; a deployment without them
renders as before.

The network is said with three more keys (ADR-137): `zone` on a server,
IEC 62443's word, a kebab-case name for servers that share security
requirements; `listens` on a server, each a `port` and the `protocol` on
it; and `links` on a deployment, each a firewall rule in NIST SP 800-41's
form, `from` a named server `to` a named server on a `port` the target
listens on, with a `description` when the reason is not plain. A link
the other way is another link.

    servers:
      - { name: api, url: "https://library.example", zone: application, listens: [{ port: 8080, protocol: http }], runs: { packages: [internal/api] } }
      - { name: db, kind: database, url: "postgresql://db.internal:5432", zone: data, listens: [{ port: 5432, protocol: postgresql }], runs: { targets: [sql] } }
    links:
      - { from: api, to: db, port: 5432 }

A link that names no server of the deployment, joins a server to itself
or names a port the target does not listen on is an error at the entry
(`link`). Once a deployment declares `links`, the validator runs the
firewall check and warns at a server (`link_missing`) whose packages
implement an operation or a job that calls a dependency, or emits on or
consumes from a channel a dependency carries, with no link to the server
that is that dependency in the deployment; and at one whose packages
implement an entity, or a job that reads or writes one, with no link to
the deployment's database server that runs the `sql` target. An open
must question that blocks the environment the deployment installs
(`#/environments/production`), the `environments` section or the
`deployment` stage holds the warning, and the `link` errors stay
(ADR-172). A job is
placed by the package that implements it (`implements:
["#/jobs/markOverdue"]`). The documents then draw the deployment as its
network: a subgraph per zone, an edge per link with its port and the
target's protocol, and the dependencies no server is apart; the marker
`network <deployment>` draws it in a hand-written `specarch.md`. The
handover document prints the links and the ports before the
infrastructure's sentences, which say what the keys do not.

Pointers from an implementation file into its specification are JSON
pointers written from the merged document's root: `#/entities/Loan`,
`#/commands/validate`, `#/paths/~1loans/post` (a `/` inside a key is
written `~1`). The validator checks that `implements` names the root file
at the same `info.version` and that every pointer resolves, so a design
change is noticed by every implementation of it.

Targets read the specification for the design and the implementation file
for the target choices: which folder a target owns, the settings of the
code generator that follows it, the type a decimal maps to.

An operation's mapping may hold what its handler does, as `specarch extract
go` read it from the code (ADR-144), under `handler`: the commit read, the
depth followed, and `functions`, the handler first and then each function
of the same code its steps reach, each with its name, file, first and last
line, its `steps` and its `source` as it was at that commit, each line
without its trailing spaces and tabs and without the first line's
indentation. A step has an `id` (`s` and its
number in the order written), a `kind` (`input`, `branch`, `choice`,
`loop`, `database`, `transaction`, `call`, `external`, `response`,
`return`, `statement`), its `text` in plain words, its `file`, `firstLine`
and `lastLine`, and what its kind adds: `parameter`; `then` and
`otherwise`; `cases` of `when` and `steps`; `body`; `table`, `access` and
`transaction`; `function` or `stopped`; `dependency`; `status` and
`problem`. The record is implementation because what a handler does on one
stack is that stack's; the design says what the operation answers. The
technical specification draws each function as a flowchart whose nodes
carry the steps' ids, with the source under it, and the validator warns
`handler_source` when the source a record holds is no longer what the
cited file holds at the function's lines, read from the specification's
root through the code source's `url`. `specarch extract go
--implementation-out` writes the records into a fragment, and given
`--implementation` prints how each handler's steps changed since the
file's record.

An element another stakeholder owns, such as a table another team keeps
in the same database or a route another service answers behind the same
gateway, is marked on its mapping with `ownedBy`, naming a stakeholder of
the specification:

```yaml
mappings:
  "#/entities/Book":
    target: table books
    ownedBy: catalogue-team
```

The specification still describes the element: validate, diff, gaps, the
documents and the generation gate read it as they read any other, and so
do the generated tests, which check the running system. No generator that
builds code or data writes it, nor anything under its pointer; an element that
refers to it, such as a foreign key or a schema reference, still refers
to it. The mark is per mapping because the parts another team owns are
rarely a whole file, and it sits in the implementation file because who
builds what is a choice of one implementation, not part of the design
(ADR-046). An `ownedBy` that names no stakeholder is the error
`stakeholder`.

### The interface boundary

The rule of thumb: if a client of the interface needs to know it, it is
design and goes in the specification. If only the people building or
running one particular implementation need it, it goes in the
implementation file.

| Specification (design) | Implementation file (one stack) |
|---|---|
| the kind of interface: HTTP (`paths`), messaging (`channels`), command line (`commands`) | the framework binding: router, middleware, how handlers implement the generated interface; the CLI library |
| operations with operationId, method and path; parameters, request bodies, responses with schemas, status codes and error shapes | code-generator configuration: package names, strict-server mode, `x-oapi-codegen-*` extensions, protoc plugins |
| channels and messages | the messaging client and its settings |
| commands, arguments, options and exit codes | |
| the permission each operation, command and page needs; roles | |
| environments, what each is for and who runs it; the settings and which are secret; release, rollback, backup, restore and migration steps; the handover to a customer who runs an environment | servers, hosts, ports and the values of the non-secret settings per deployment; the exact commands; how a release is built (the task, the base images by digest, the software bill of materials), how it is installed and what the hosts must provide |
| the commissioning checks and the sign-off | |
| the monitors: what is measured, in which environment, and the objective | the tool that measures, how, and where an alert goes, per monitor in each deployment |

A standalone `openapi.yaml` or `asyncapi.yaml` is made from the
specification. It is never a third source kept by hand.

Both schemas enforce the boundary. The specification schema refuses extension
keys that name one implementation: `x-oapi-codegen-*`, `x-go-*` and the
other language prefixes, `x-server`, `x-host`, `x-port`, `x-deploy`,
`x-router`, `x-framework` and the rest of the `stackSpecificKey` pattern in
the schema. The implementation schema is closed and has no design keyword.
The validator reports each case with its own rule, `stack_key` or
`design_key`. Other interface kinds (gRPC, file exchange, a serial protocol)
have no keyword yet; they are items in `docs/roadmap.md`.

### No change log inside a file

A specification or implementation file says what is true now. Text such as
"previously", "changed from", "updated on" or "new in" does not belong in a
description; the history of a project lives in its own history files and in
git. Version fields stay, because they describe the current file. The
validator warns on the most common change-log phrases (`change_log`).

## Why, citations and origin

Every element at every stage may carry three optional fields, the same
everywhere:

- `why`: the rationale in plain words, why the element is the way it is
  and what was concluded. A document renders it as an Insight next to the
  element.
- `cites`: a list of citations, each naming a `source` declared in the root
  file, the `clause` where relevant, and what the source `says` that
  applies here. A document renders each as a Note under the Insight.
- `origin`: how the element is known. `stated`: a source says it, and
  `cites` names the source and where in it (`origin_citation` otherwise).
  `inferred`: it was concluded from evidence, and `why` says from what
  (`origin_reason`). `decided`: a stakeholder settled it, and `decidedIn`
  names the accepted decision (`origin_decision`). An element without
  `origin` was written spec-first. A specification built from sources says
  `tracksOrigin: true` in `info`; the validator then warns for every element
  of a section without an origin (`origin_missing`). A document renders the
  origin as one line, **Origin:**, before the Insight. `docs/refinement.md`
  is the design.

An Insight is one paragraph that starts with **Insight:**, a Note one that
starts with **Note:** and reads "From <title>, <edition>, clause <clause>:
<what it says>", with the source's URL after it when there is one. They
follow the description of an element that has its own heading. An element
shown as a row of a table gets them after the table, labelled with the
row's name: **Insight on LIB-5:**. A document whose Notes cite sources ends
with a Sources table of exactly those. The traceability matrix names
elements by ID only, so it carries none.

Sources are declared once under `sources` in the root file, keyed by
kebab-case name, with their `kind` (standard, regulation, document,
interview, system, code, or requirement-set, change-set or defect-set for
an external tracker, each with its `prefix`), `title`, `edition`, `author`,
`date` and `url`, `givenOutside: true` when the system's owners gave
it to parties outside, such as a published interface or a contract, which
makes `specarch merge` ask a `must` question about an element only it or
only the code has, and `language`, a BCP 47 tag such as `de`, when it is
written in a language other than English, which names the vocabulary its
words are read by (`docs/vocabularies.md`). A citation of a source that is not
declared is refused (`source`), and when the source lists its `clauses`, a
citation that names no clause, or one that is neither listed nor a part of
a listed one, is a warning (`source_clause`). A `system` is a running system that was
observed; `code` is a system's source code, read at the commit its
`edition` names, and a citation of it names a file and line, or a package
and function, as its clause. A source may list its `clauses`, each with a
`title`: the numbered sections of a document, or the files and folders of
code. `specarch gaps` then shows which elements each clause produced and
which clauses produced nothing; a citation falls under the longest listed
clause it equals or starts with, followed by a dot, a colon, a slash or a
space. `docs/from-sources.md` is the procedure that uses them. Decisions keep their context, decision and
consequences: the context is what was true and at stake, `why` the
reasoning that led from it to the decision. A decision that answers open
questions names them under `answers` and the stakeholder who decided under
`decidedBy`; an answer given in words is declared as a source of kind
`interview` with its date. A decision that answers a question only in
part names the questions it raised under `opens`.

## Open questions

What the sources do not say is not invented and not left blank: it is an
open question, in the section `questions`, the one section every stage
folder may hold. A question is written in the folder of the stage it is
about: beside the elements it blocks, as `questions.yaml` or under a
`questions/` sub-folder, and as a file directly under `tests/` or
`implementation/`, whose other entries are folders. A stage with no folder
has its questions in the root file.

    questions:
      Q-12:
        question: What is the loan period for a reference copy?
        kind: decision
        priority: must
        blocks: ["#/entities/Loan/properties/dueOn"]
        decidedBy: head-librarian
        options: ["14 days, as for an ordinary copy", "Reference copies are not lent"]
        why: The policy sheet gives periods for ordinary copies only.

Each question says what is asked, its `kind` (`decision`, or `material` to
be provided), its `priority` (`must`: what it blocks is not defined;
`should`: what it blocks is inferred and must be confirmed; `could`: the
answer would help but nothing waits), what it `blocks` (a stage, a section,
or a pointer to an element or to one key of it), who decides (`decidedBy`,
a stakeholder key), and the `options` when the answer is a choice. A
question a reader asks about code it checked against the specification
also says its `certainty` (ADR-144): `certain` when the code as written
cannot work or disagrees with the specification, `suspected` when a run or
a person must confirm it; the problems file, the SARIF log and the
documents show it beside the priority and the kind. The ID
is an upper-case prefix of one or more letters, a dash and a number.

The question is the one licence for an element to be incomplete. A
required key missing at or under a pointer a `must` question blocks is
covered: the validator reports neither it nor the warnings about that
element, the idiom checks skip it, and `specarch gaps` lists the missing
keys under the question. A pointer to an element covers what is missing at
or under it, whether the element is written out in part, such as a field
with its width and no `type`, or known only by name: an element known only
by name, such as an entity or one of its fields, is written as an empty
mapping and blocked by pointer. A pointer to one key that is not there,
such as `#/workflows/<name>/trigger`, covers that key alone; when the key's
value decides what else the element needs, such as a field's `type` or a
constraint's `kind`, block the element. Covered too are a permission no
role grants when the question blocks that permission, a response that
names no problem while problem types are declared when the question
blocks its problem, and a schema or a view named like an entity, or a
schema like a view, when the question blocks that schema or view, as
`specarch merge` writes and asks it (ADR-132). Nothing else is covered: a
wrong value beside the gap is still an error, and a `should` question
covers no missing key.

The validator checks that every `blocks` entry resolves (`question_block`),
that a question sits in the stage of what it blocks and blocks one stage
(`question_stage`), that `decidedBy` is a stakeholder (`stakeholder`), and
that no accepted decision `answers` a question still present
(`question_answered`), and that every question a decision `opens` is open
or answered by a decision, and is not one that decision answers itself
(`question_opened`). `validate` counts the open questions in its summary
line and lists none; `specarch gaps` and the document `questions` list them
by stage, with what each holds up and which outputs are ready, drafts or
waiting, each output's questions counted per stage with the first three
named. A question of kind `material` asks a stakeholder to provide
something to read rather than to decide, so both list it in a group of its
own, Material to provide, with whom it is needed from, and count it apart
from the decisions each stakeholder must take. A question is removed when
it is answered; the answer is a decision with `answers` and `decidedBy`,
and the elements it settled carry `origin: decided`. A decision that
answers a question only in part keeps the open half as a question of its
own and names it under `opens`; `gaps` shows that question with the
decision that opened it and the questions the decision answered, back
along the chain.

## YAML layout

Top-level keys appear in this order, which is life-cycle order. A target
does not care; a reader does. In a tree, each section is in its stage's
folder and only the first four are in the root file.

1. `specarch`, `info`, `stages`, `sources`
2. `stakeholders`, `needs`, `requirements`, `glossary`, `assumptions`, `constraints`
3. `enums`
4. `entities`
5. `permissions`, `roles`, `session`
6. `paths`
7. `commands`
8. `channels`, `dependencies`, `jobs`, `gateways`, `workflows`, `errors`
9. `pages`, `menus`
10. `algorithms`
11. `tests`
12. `decisions`
13. `environments`, `configuration`, `release`, `rollback`, `migrations`
14. `checks`, `signoff`
15. `monitors`

Inside an entity: `description`, `type`, `properties`, `required`,
`primaryKey`, `relations`, `constraints`, `stateField`, `transitions`,
`satisfies`, `why`, `cites`. Fields appear in the order they would be shown
on a form: identity first, then the fields a person fills in, then the
fields the system sets.

Short objects are written on one line with flow style:
`{ type: string, format: uuid, readOnly: true }`. Anything with a
`description` longer than a few words goes block style. Prose longer than one
line uses a literal block (`|`).

Three values are always quoted, because YAML would otherwise read them as
something else: dates (`date: "2026-10-07"`, which YAML takes for a
timestamp), decimal amounts (`"0.50"`, which YAML takes for a float), and any
flow-style string containing a comma, a colon or a question mark. The schema
rejects the unquoted forms, so the mistake is caught, but it is better not to
make it. A `says` or `statement` with a comma in it is written in block
style or in double quotes.

### Names

| Thing | Case | Example |
|---|---|---|
| entity, enum, message | PascalCase | `Loan`, `LoanStatus`, `LoanCreated` |
| field, relation, algorithm, operationId, input, setting | camelCase | `dueOn`, `createLoan`, `dailyRate` |
| enum value | snake_case | `set_null`, `overdue` |
| constraint of an entity | snake_case, prefixed with the entity | `loan_due_after_loaned` |
| permission | dotted lower-case, `area.verb` | `loans.create` |
| role, page, stakeholder, source, environment, migration, check, test | kebab-case | `librarian`, `members-list`, `iso-29148`, `production` |
| channel | dotted lower-case | `loan.lifecycle` |
| command | kebab-case words separated by spaces | `validate`, `document techspec` |
| command argument, option | kebab-case | `paths`, `out` |
| decision | `ADR-` and three or more digits | `ADR-001` |
| requirement, need, assumption, constraint | an upper-case prefix, a dash and an ID | `LIB-5`, `NEED-1`, `ASSUME-1`, `CON-1` |
| glossary term | as written | `late fee` |

The schema enforces these patterns. Targets rely on them to derive names in
the output (table names, URL segments, constant names) without a mapping table.

A field keeps its camelCase name in the specification however the
interface writes it. An API whose JSON names its properties in snake_case
says so once, with `info.wireNames: snake_case`: every property of every
entity, view, body, parameter schema and message payload then goes on the
wire in snake case (`loanedOn` is `loaned_on`, `userID` and `userId` are
both `user_id`), and the validator refuses two properties of one object
that would meet there (`wire_name`). A parameter's name is written as it
is on the wire, and is not mapped (ADR-062).

A role keeps its kebab-case name in the specification in the same way. A
system whose token's roles claim and permission tables write a role in
upper snake case says so once, with `info.roleNames: upper_snake`: every
role then goes on the wire as its name in capitals with each hyphen an
underscore (`desk-supervisor` is `DESK_SUPERVISOR`), and the validator
refuses two roles that would meet there (`wire_name`, ADR-099).

In SQL, a table is the entity's name in snake case (`LoanStatus` is
`loan_status`), unless the implementation file's mapping names it
(`target: table loans`); a column is the field's name in snake case
(`dueOn` is `due_on`). A primary key is `pk_<table>`, the check of a
type-rendering row `ck_<table>_<column>`, a foreign key
`fk_<table>_<column>`, and a declared unique or check constraint keeps its
own name. A plain index under a mapping's `indexes` is
`ix_<table>_<column>_<column>`, its columns in its order, unless it names
its own. A row security policy of a role's `rows` rule is
`rls_<table>_<role>`, the role's hyphens as underscores, and applies to
the database role the mapping of `#/roles/<name>` gives, or the role's
name with underscores. Identifiers are written unquoted and in lower case.

### Types

Every value has a concrete type, and the type says how wide it is. A design
that only says "number" leaves each implementation to guess, and the guesses
differ: in JavaScript every number is a 64-bit float, so an int64 ID above
2^53 silently loses its last digits, and `1` and `"1"` are easy to mix up.
Stating the concrete type is what makes a correct implementation possible.

A field's `type` says how the value travels in JSON; its `format` says what
the value is. The types, one line each:

| Type | Written as | Holds |
|---|---|---|
| int32 | `type: integer, format: int32` | a signed 32-bit integer; safe as a JSON number everywhere |
| int | `type: integer, format: int64` | a signed 64-bit integer, as a JSON number; needs `minimum` and `maximum` within plus or minus 2^53 - 1 (9007199254740991) |
| int, carried as text | `type: string, format: int64` | a signed 64-bit integer that may exceed 2^53, carried as a JSON string of digits |
| uint | `type: integer, format: uint64` or `type: string, format: uint64` | an unsigned 64-bit integer, with the same rule for 2^53 |
| double | `type: number, format: double` | a 64-bit IEEE float: approximate, never for money |
| decimal | `type: string, format: decimal, precision: P, scale: S` | an exact decimal of P digits, S after the point, carried as a JSON string |
| string | `type: string` | text; a string of digits (a card number, a postcode) is a string, never a number |
| bool | `type: boolean` | true or false |
| bytes | `type: string, format: byte` | binary data, base64 in JSON |
| date | `type: string, format: date` | a calendar date, `"2026-10-07"` |
| timestamp | `type: string, format: date-time` | an instant, RFC 3339, `"2026-10-07T09:30:00Z"` |
| duration | `type: string, format: duration` | a length of time, ISO 8601, `"PT2H"` |
| time of day | `type: string, format: time` | a time of day without a date, RFC 3339 partial time, `"09:30:00"` |
| enum | `$ref: "#/enums/Name"` | one of the enum's values, carried as a string |
| map | `type: object, additionalProperties: { ... }` | an object whose members are any text and whose values all have the field's type; `additionalProperties: true` for values of any type |

The schema refuses an `integer` without `int32`, `int64` or `uint64`, a
`number` without `double`, and a decimal without `precision` and `scale`.
The validator refuses an int64 or uint64 carried as a JSON number without
bounds inside 2^53 (`unsafe_integer`): either bound it, or carry it as a
string. `format` keeps its JSON Schema and OpenAPI meaning; `uint64` and
`decimal` are SpecArch's additions to the format list, because neither
standard names them.

A map is JSON Schema's `additionalProperties` on an object (ADR-125): a
field for the type of every member the object's `properties` do not name,
`true` for members of any value, `false` for none. With no `properties` the
object is a map, whose keys are data and never wire-named; with both, the
named members are fixed and the rest follow `additionalProperties`. Any
value is written `true`, never `{}`, and the keyword goes on an object
only. `patternProperties` and `propertyNames` are not in the subset: a key
pattern names no member a stack's type can hold, so a generator could only
leave it unchecked. An entity and a value object name their members, so a
map is a field of one, stored as JSON.

### Where each keyword comes from

Every keyword in the meta-model is either borrowed from a standard, with its
standard meaning, or marked as SpecArch's own. Borrowed keywords are never
redefined.

| Keyword | Origin | Notes |
|---|---|---|
| `info`, `paths`, `parameters`, `requestBody`, `responses`, `content`, `operationId`, `summary`, `deprecated` | OpenAPI 3 | bounded subset: `in` is path, query or header; no callbacks, links or servers |
| `type`, `title`, `properties`, `required`, `additionalProperties`, `enum`, `const`, `format`, `default`, `examples`, `minimum`, `maximum`, `exclusiveMinimum`, `exclusiveMaximum`, `multipleOf`, `minLength`, `maxLength`, `pattern`, `items`, `minItems`, `maxItems`, `uniqueItems`, `readOnly`, `writeOnly`, `$ref` | JSON Schema 2020-12 | `$ref` is restricted to `#/entities/X`, `#/enums/X`, `#/views/X` and `#/schemas/X`, of this specification or, as `<dependency>#/...`, of another one a dependency names (References between specifications); nullability is written `type: [string, "null"]` as JSON Schema does |
| `channels`, `messages`, `payload` | AsyncAPI | one level: channel, messages, payload; no servers, bindings or operations objects |
| `x-*` | OpenAPI convention | allowed in every object, ignored by validation |
| `stages`, `sources`, `givenOutside` | SpecArch | the root file's list of stage folders and its registry of cited sources, each marked when it was given to parties outside |
| `language` (on a source) | BCP 47 | the language subtag and the subtags after it; the tag names the vocabulary a document is read by |
| `stakeholders`, `needs`, `requirements`, `statement`, `priority`, `status`, `acceptance`, `verification` | SpecArch, after ISO/IEC/IEEE 29148 | the requirements stage: the standard's stakeholders (5.2.2), needs (6.3), requirements (6.4) and attributes (5.2.8); priority after MoSCoW; verification methods after MIL-STD-961E |
| `glossary`, `assumptions`, `constraints` (top level) | SpecArch, after ISO/IEC/IEEE 29148 | 9.2.3, 9.5.19 and 9.6.16 of the standard; arc42 sections 2 and 12 |
| `satisfies`, `verifies` | SpecArch, after SysML and 29148 6.5 | the satisfy and verify relations from a design element or a test to a requirement |
| `why`, `cites`, `source`, `clause`, `says` | SpecArch | the rationale and the citations every element may carry; 29148 (5.2.8) names rationale and source as attributes |
| `primaryKey`, `relations`, `constraints`, `stateField`, `transitions` | SpecArch | data-model concepts JSON Schema has no words for |
| `where` (of a unique constraint) | SpecArch, after PostgreSQL's partial index and SQL Server's filtered index | the records a unique constraint holds among; ISO SQL leaves indexes out, so no standard names it |
| `enabledBy` | SpecArch | the boolean setting that switches an operation, command or page on |
| `release` (of a requirement) | SpecArch | the release a requirement is meant for, by the version of its release record, as a change or a defect names the release it shipped in |
| `fulfilledBy` | SpecArch, after JSON Schema's `$ref` as a URI reference | beside `ownedBy` on a requirement, the requirement of another specification that fulfils it, `<dependency>#/requirements/<id>` (ADR-119) |
| `audience` | SpecArch | `internal` on an element for the people who build and run the system only; left out, it is for everyone, and `document --audience external` leaves the internal elements out and counts them (ADR-119) |
| `generatedDefault` | SpecArch | a default the database computes, which `default` cannot hold since it is no value: `identifier`, a new random UUID, on a field of format `uuid`, or `currentTime`, the instant the record is written, on a `date-time`; never beside `default`. The type-rendering idiom names each dialect's expression (ADR-121) |
| `precision`, `scale` | SpecArch | decimal size; JSON Schema has no decimal type, so `format: decimal` on a string carries them |
| `valueDescriptions` | SpecArch | per-value meaning of an enum |
| `commands`, `arguments`, `options`, `reads`, `writes`, `standardOutput`, `standardError`, `exitCodes`, `remedies`, `repeatable`, `info.program` | SpecArch | command-line interfaces; no standard describes one. `info.program` is the name typed before a command's words, `remedies` what the person does after each status a command fails with (ADR-165) |
| `permissions`, `roles`, `permission` | SpecArch | OpenAPI's `security` names a scheme, not a right; SpecArch needs the right |
| `caller` | SpecArch | the one record of who is making the call, with properties as JSON Schema has them; `caller.<property>` in an expression is CEL's own field access, kept to this one record so that every name is still a declared column of one record and a generator renders a rule without a join (ADR-095) |
| `clients`, `kind` (of a client), `softwareVersion`, `softwareStatement`, `roles` (of a client) | SpecArch, after RFC 7591 and RFC 6749 | a client keyed by its `software_id`, with the RFC's `software_version` as an admitted range and its `software_statement` as whether the version is the publisher's; `client_id` is a deployment value (ADR-096, ADR-106) |
| `proof`, `signedRequest`, `algorithm`, `covers`, `mutualTls` | SpecArch, after RFC 9421 (HTTP Message Signatures) and RFC 8705 | how a client proves its identity on each request; the algorithm names and covered components are the RFC's (ADR-096) |
| `callers`, `people` | SpecArch | the explicit subset of who may call an operation or command; `people` reserved for anyone signed in (ADR-110) |
| `nobody` | SpecArch | the reserved permission of a route closed on purpose, beside `public` (ADR-096) |
| `through` (of a role) | SpecArch, after RFC 8693's `client_id` claim | the clients a grant holds through, checked against the application the token carries (ADR-106, ADR-108) |
| `tokens`, `format`, `lifetime`, `signature` (of a token), `claims`, `checks`, `default`, `token` (on an operation) | SpecArch, after RFC 7519, RFC 9068, RFC 7518 and RFC 8725 | a token format: its kind, lifetime, algorithm, claims as properties, checks over them and whether it is the default (ADR-097) |
| `trustDomain` | SpecArch, after the OAuth Transaction Tokens draft | in the identity module's root file, the URI naming the trust domain: the `aud` of every transaction token and the base of every security event's identifier (ADR-107) |
| `<dependency>#/permissions/<name>`, `<dependency>#/clients/<name>`, `<dependency>#/roles/<name>` | SpecArch | a permission, client or role of another module, named through a dependency: a role of the identity module grants each module's permissions, and `via`, `callers` and a test's caller name the identity module's clients and roles (ADR-107) |
| `tokenType`, `via` | SpecArch, after the OAuth Transaction Tokens draft | `txn_token` is the suffix of the draft's token type identifier; `via` names the immediate callers of a hop (ADR-111) |
| `iss`, `sub`, `aud`, `exp`, `nbf`, `iat`, `jti` | RFC 7519 | registered claims, written as the registry names them |
| `act`, `scope`, `client_id`, `may_act` | RFC 8693 | registered claims |
| `cnf` | RFC 7800 | registered claim |
| `txn`, `events`, `toe` | RFC 8417 | registered claims |
| `auth_time`, `acr`, `amr`, `azp` | OpenID Connect Core | registered claims |
| `req_wl`, `rctx`, `tctx` | the OAuth Transaction Tokens draft | its claims, pending registration |
| `securityEvents`, `store`, `alarm` | SpecArch, after RFC 8417 and OWASP ASVS 5.0 V16 | where every refusal's security event goes (ADR-109) |
| `rows` (of a role) | SpecArch, after PostgreSQL's `CREATE POLICY ... USING (...)` | the rows a role reaches in an entity, one bool expression per entity, permissive across roles as the policies are; ISO SQL has no row security, so the one engine that models it names it (ADR-095) |
| `tenant`, `property`, `key`, `shared` | SpecArch | the tenant scope said once: the caller's property, the field every tenant-owned entity carries and the entities every tenant reads whole, from which every role's rows rule over those entities is derived; no standard names a tenant scope, and PostgreSQL's policies are still written one per role and table, which is what the derived rules become (ADR-179) |
| `separationOfDuties`, `cardinality` | ANSI INCITS 359 (RBAC), static separation of duty | a set and a number no holder may reach; stated over permissions, not roles, because the specification declares the duties and the roles that grant them |
| `session`, `idleTimeout`, `absoluteTimeout` | SpecArch, after the OWASP Session Management Cheat Sheet | how a signed-in session ends; the two timeouts are the cheat sheet's |
| `signIn`, `provider`, `client`, `flow`, `pkce`, `callback`, `identityToken`, `logout`, `backChannel` | SpecArch, after OpenID Connect Core 1.0, RFC 6749, RFC 7636 and OpenID Connect Back-Channel Logout 1.0 | how a person signs in at an identity provider: the authorisation code flow with PKCE, the callback of this system, the identity token and the provider's back-channel logout (ADR-182) |
| `nonce` | OpenID Connect Core | a claim of the identity token, written as the standard names it |
| `middlewares`, `refusals` | SpecArch | the steps a request passes before an operation's handler, declared once with what each checks or does and the refusals it answers, each a `status`, a `problem` and a `condition`; an operation lists the ones it runs, in order, `[]` for none. No standard names them: OpenAPI's `security` says how a caller proves who it is, not what runs (ADR-131) |
| `dependencies`, `timeout`, `calls` | SpecArch | external systems an operation calls, each with a time limit per call |
| `kind` on a dependency, `carriedBy` on a channel | SpecArch | what a dependency is (five kinds, after C4's software system and containers), and the dependencies a channel lives on; AsyncAPI's channel `servers` names hosts, which a design file excludes (ADR-134) |
| `protocol` on a dependency | AsyncAPI, the server object | how the dependency is reached, a lowercase URI scheme name, kept open |
| `dependsOn` on a layout entry, in an implementation file | CycloneDX, a component's dependencies | the other packages of the layout this one calls, imports or links against (ADR-133) |
| `name`, `kind`, `runs`, `dependency` on a deployment's server, in an implementation file | SpecArch, after C4's deployment node and container instance | what a server is, what of the implementation runs on it, and which dependency it is in that environment; C4's words name the thing placed, not the placing (ADR-136) |
| `zone` on a server, in an implementation file | IEC 62443-3-2, the zone | a grouping of servers that share security requirements (ADR-137) |
| `listens` on a server, `links` on a deployment, in an implementation file | NIST SP 800-41, a firewall rule | what a server accepts, and the directed connections allowed between servers on a port; IEC 62443's conduit joins zones, and the check needs the host and the port (ADR-137) |
| `specification`, `version` (on a dependency), `name` (in `info`) | SpecArch, after JSON Schema's `$ref` as a URI reference | a dependency that is another specification, named by its `info.name` and pinned to its `info.version`; its elements are named `<dependency>#/<pointer>` |
| `specarchEstate`, `modules`, `location` | SpecArch | an estate file: the modules' specifications by `info.name`, each with its folder and the version held |
| `idempotencyKey` | SpecArch, after RFC 9110 9.2.2 and the IETF Idempotency-Key header draft | a header that tells a repeated request from a new one, on a method that is not idempotent by itself |
| `validity`, `from`, `until` | SpecArch | the fields that bound when a record is current |
| `guard`, `precondition`, `recordsChanged` | SpecArch | what is checked together with a data change |
| `writes`, `set`, `delete`, `given` | SpecArch | what a guarded change writes when no keyword of its entry says it: the fields it sets, each to the value the operator gives or a literal, or that it deletes the records; no standard says what an operator's correction writes (ADR-168) |
| `sensitivity`, `atRest`, `lookup`, `audited`, `deletion` | SpecArch | how sensitive a field is and how it is stored, and what the system keeps about each record of an entity |
| `listOf`, `searchable`, `filterable`, `sortable`, `pageSize` | SpecArch | a list's whitelists and page size, which a client must know |
| `limits`, `maxRequestBytes`, `maxResponseBytes`, `rate`, `requests`, `per`, `burst` | SpecArch | the request size and rate a client keeps to, and the largest download it must be ready for |
| `handshakes`, `keyAgreement`, `signature`, `cipher`, `establishedBy`, `lifetime`, `uses`, `handshake` | SpecArch, after RFC 7748 (the key agreements), RFC 8032 (the signatures) and OpenSSL's cipher names | a key exchange a client performs once, after which the operations that name it travel in an encrypted, signed envelope; declared once, named by each operation that travels in it (ADR-145) |
| `stream`, `direction` | SpecArch | an operation whose request body (`upload`) or success response (`download`) is a file passed through as it arrives, never read whole; the media type and the binary string stay in OpenAPI's `requestBody` and `responses` (ADR-146) |
| `jobs`, `trigger`, `schedule`, `every`, `consumes`, `role`, `retries`, `limit`, `then` | SpecArch | work the system does on its own; a schedule is the five fields of cron, in UTC |
| `gateways` | SpecArch, after RFC 9110's gateway (section 3.7) | an intermediary that forwards every request no operation answers, as received, to the service a claim of the verified token names; it names no permission (ADR-141) |
| `claim`, `value` on a gateway | SpecArch, after RFC 7519's claim | the claim whose value selects the service, and one value of it; for `aud` an absolute URI (RFC 8707, section 2) |
| `forwards` | SpecArch | a gateway's table of services, one row per value with the dependency it selects |
| `token`, `calls`, `middlewares`, `refusals`, `dependency` on a gateway | SpecArch | as on an operation, a middleware and a server: the token verified, the dependencies called besides the forwards, the middlewares run first, the answers given in place of the service's, the dependency a row forwards to |
| `menus`, `title`, `page`, `items` | SpecArch | the navigation: a tree whose leaves open pages |
| `workflows`, `trigger`, `subject`, `steps`, `approvers`, `deadline`, `onDeadline`, `escalateTo` | BPMN 2.0, a sequential subset | a request that finishes after people approve it: an approval is a user task with its potential owners and a timer, an operation step a service task |
| `views`, `from`, `path`, `count`, `rows` | SpecArch, after the SQL view of ISO/IEC 9075 | a read model: an entity's row with fields read through its relations, counts and the rows of a one-to-many relation added, never written |
| `schemas` | OpenAPI 3, `components.schemas` | value objects: named object schemas with no key and no table, data passed around but not stored |
| `errors`, `status`, `title`, `condition`, `remedy`, `type`, `problem` | RFC 9457, Problem Details for HTTP APIs | the catalogue of problem types; `condition` and `remedy`, what the person who meets the problem does next (ADR-160), are SpecArch's, the standard's other members are the document's own at run time |
| a duration (`timeout`, `idleTimeout`, `absoluteTimeout`) | ISO 8601, the form JSON Schema's `format: duration` names | days, hours, minutes and seconds only (`PT5S`, `P1DT12H`): weeks, months and years depend on the calendar, so a limit written in them would not mean the same every day |
| `emits`, `algorithm` | SpecArch | links from an operation to its events and its computation |
| `pages`, `kind`, `route`, `entity`, `source`, `submit`, `columns`, `fields`, `filters`, `actions` | SpecArch | UI page definitions; a page of kind `task` submits to an operation without loading a record |
| `onSubmitted`, `onSelect`, `then`, `navigate`, `with`, `message` | SpecArch, after the events and navigation flows of OMG IFML 1.0 | where an event of a page leads, and the status message it carries (WCAG 2.2, 4.1.3); `navigate: signIn` leaves for the identity provider's sign-in (ADR-182) |
| `flows`, `actor`, `steps`, `event`, `action` | SpecArch, after IFML's navigation flows | a task a person does across pages, step by step |
| `states`, `empty`, `filteredEmpty`, `failed`, `message`, `field` | SpecArch | what a page shows when it is empty or fails |
| `sections`, `title`, `fields` | SpecArch | a form's or a view's fields in titled groups, in reading order |
| `compactColumns` | SpecArch, after the compact size class of Apple's Human Interface Guidelines and Material Design 3 | the columns a list keeps on a compact screen |
| `childRows`, `relation`, `maximum`, `lockLoadedRows` | SpecArch | the records of a one-to-many relation edited as rows under a form |
| `accessibility`, `standard`, `level` | WCAG 2.2 | the accessibility the user interface conforms to |
| `theme`, `tokens`, `$type`, `$value`, `$description`, `colorSpace`, `components`, `alpha`, `hex`, `unit` | W3C Design Tokens Community Group, Format Module 2025.10 | the visual design as design tokens |
| `modes`, `pairs`, `text`, `background`, `use` | SpecArch | the values a mode gives tokens, and the colours shown together, whose contrast is checked |
| `algorithms`, `inputs`, `output`, `formula`, `examples` (of an algorithm), `pseudocode` | SpecArch | IEEE 1016 algorithm viewpoint, made testable |
| `decisions` and the ADR fields | SpecArch, after Michael Nygard's record | the common ADR shape: context, decision, consequences, plus `why` |
| `opens` (of a decision) | SpecArch | the questions a decision raised, by ID, the open half of an answer given in part; each is open or answered by a later decision (ADR-120) |
| `tests`, `scenario`, `level`, `given`, `when`, `then`, `covers`, `notApplicable` | SpecArch, after ISO/IEC/IEEE 29119 and Gherkin | design tests; the levels are 29119-1's; given, when and then are the Gherkin words, without Gherkin's file format |
| `environments`, `promotesTo`, `configuration`, `secret`, `release`, `rollback`, `migrations`, `steps`, `action`, `check` | SpecArch, after ISO/IEC/IEEE 12207 6.4.10 | the deployment stage: the transition process, written as environments, settings and steps |
| `runBy`, `handover`, `to`, `source` (of a handover), `responsibilities` | SpecArch, after ISO/IEC/IEEE 12207 6.1.1 and 6.1.2 | who runs an environment, the vendor or the customer (12207's supplier and acquirer), and what a customer who runs one receives (ADR-129) |
| `checks`, `signoff`, `criteria`, `signers` | SpecArch, after ISO/IEC/IEEE 12207 6.4.11 and IEC 62381 | the commissioning stage: validation on the installed system, the site acceptance test |
| `monitors`, `objective`, `response` | SpecArch, after ISO/IEC/IEEE 12207 6.4.12 | the operation stage: what is watched on the live system, and the steps the operator takes when the objective is missed (ADR-160) |
| `backup`, `restore` | SpecArch, after ISO/IEC/IEEE 12207 6.4.12 | the operation stage: how the live system's data is saved and put back, as procedures (ADR-129) |
| `testing`, `suites`, `designTests`, `designTestsOf`, `implementationOnly` | SpecArch | in implementation files: how one stack runs the design tests |
| `stack`, `targets`, `deployments`, `environment` (of a deployment) | SpecArch | in implementation files: the stack, the output folders per target, and where the system really runs |
| `ownedBy` | SpecArch | the stakeholder that owns what someone else builds: on a requirement, the one whose component fulfils it, so nothing here satisfies or verifies it (ADR-119); in implementation files, on a mapping, the one that owns an element the project describes but does not generate |
| `permissionChecks`, `package`, `function`, `permissionArgument` | SpecArch | in implementation files: on a binding, the project's own permission checks, which `specarch extract` reads routes' permissions through (ADR-081) |
| `middlewares` (in implementation files) | SpecArch | on the http binding, the function of the code each middleware of the design is, by its name: its `package` and `function` (ADR-131) |
| `indexes` | SpecArch | in implementation files: on an entity's mapping, the plain indexes of its table, each its `fields` and, when the database already names it otherwise, its `name`; an index is physical design, so it stays out of the design file, and a unique index is the entity's unique constraint (ADR-121) |
| `listRoutes`, `postSegment` | SpecArch | in implementation files: on the http binding, that every list is served two ways, by GET and by POST at its path with `postSegment` added, its parameters in a body; `generate openapi` writes the POST twin of each list (ADR-121) |
| `clientChecks`, `clientArgument`, `signatureChecks`, `applicationChecks`, `applicationArgument`, `callerChecks`, `callerArgument`, `audienceChecks`, `audienceArgument`, `eventWriters` | SpecArch | in implementation files: on the http binding, the project's own checks of the clients, a request's signature, the application the person signed in through, the immediate caller and a token's audience, each with the argument that carries the names, and the functions that write a security event, which `specarch extract go`, `swift`, `javascript` and `dart` read (ADR-154, ADR-159, ADR-169) |
| `bypassedFor` | SpecArch | in implementation files: on a permission check, the kind of caller it lets through whatever its roles grant, `client`, which `specarch extract go` asks about at every operation the check guards (ADR-114) |
| `schemaFile` | SpecArch | in implementation files: on the ui binding, the name of the file beside a page's file that holds its schema, by the project's convention, which `specarch extract pages` reads and `specarch-gen-ui-typescript` writes; `page.schema.ts` when left out (ADR-115) |
| `components`, `component`, `binds`, `nameProperty` | SpecArch | in implementation files: on the ui binding, the components of the project's UI library that bind a field or name a column, which `specarch extract` reads a screen's fields and columns through (ADR-091) |

### Traceability links

A design or deployment element names the requirements it meets under
`satisfies`: entity, enum, relation, constraint, transition, permission,
role, operation, command, channel, message, page, algorithm, decision,
environment, setting, release, rollback, migration, and an implementation
decision. A test, a commissioning check or a monitor names the ones it
shows to be met under `verifies`. A requirement names the needs it refines under `needs`. A
field, a parameter, a response, an action or a worked example carries no
link; each traces through the object that holds it. Every ID must be a
requirement of the specification, or belong to an external set declared
under `sources` with kind `requirement-set` and its prefix (`requirement`).

The lists are optional, because a specification is written in order and a
project may not have written its requirements down. Once they are written,
the validator warns for every gap: a need no requirement refines, unless
its status is rejected (`need_unrefined`), a requirement without acceptance criteria
(`acceptance_missing`), a requirement nothing satisfies once the
specification has a design (`requirement_unsatisfied`), and a requirement
nothing verifies once it has tests, checks or monitors (`requirement_unverified`).
Chapter 13 of the techspec is the matrix.

A requirement another stakeholder's component fulfils is held elsewhere
(ADR-119): it carries `ownedBy`, a stakeholder of the specification
(`stakeholder`), and may name the requirement of another specification that
fulfils it, `fulfilledBy: <dependency>#/requirements/<id>`, which is checked
as any reference into another specification once that one is given in the
same run (`reference`), and counted as unresolved otherwise. Nothing here
need satisfy or verify it, so it draws neither `requirement_unsatisfied`
nor `requirement_unverified`; it still needs acceptance criteria. The
traceability document lists it under Requirements held elsewhere.

### Audience

An element written for the people who build and run the system only, such
as a note from inside the team or a requirement written as a proposal below
the promises, says `audience: internal` (ADR-119); left out, an element is
for everyone. The key goes on any element named by a key, and an element
inside an internal one is internal with it. An element for everyone may not
name an internal one through a `$ref`, a relation's `target`, a page's
`entity` or a requirement's `needs` (`audience`), since a document for
everyone would then name what it may not show. `specarch document
--audience external --out <folder>` writes a document for everyone: the
internal elements are left out, with the links, implementation mappings,
open questions and problems that concern only them, and its summary counts what it left out in each
section. Without the flag, a document holds everything and marks each
internal element.

### Access control is fail-closed

Every operation, command and page names exactly one `permission`. Leaving it out
is a schema error. The permission `public` is reserved for things open to
everyone and must be written out; there is no default. A role lists the
permissions it grants and cannot be empty. A permission may list `implies`,
the permissions its holder holds too, transitively (ADR-100): `v1.all`
implying `v1.get` lets a holder of `v1.all` call what `v1.get` guards. Every
check of what a role grants counts what its permissions imply, and the
techspec's matrix marks an implied one with the permission it comes through.
An undeclared name, `public` on either side and a cycle are refused
(`implies`).

### The caller and the rows a role reaches

A specification may declare the caller: who is making the call, as the
service knows it once the session or token has been checked, as one record
with the properties of a value object (ADR-095):

    caller:
      description: Who is making the call, once the session has been checked.
      properties:
        memberId: { type: [string, "null"], format: uuid, description: The member signed in; null for staff. }

A role may then limit the rows it reaches in an entity, with one expression
per entity over that entity's fields and the caller's properties, written
`caller.<property>`, giving true or false in the subset of Expressions
below:

    roles:
      member:
        description: A card holder.
        permissions: [catalogue.read, loans.read]
        rows:
          Loan: memberId == caller.memberId

A holder of the role reads and changes only the records the rule holds for;
an entity not listed is reached whole; a caller holding several roles
reaches the union of their rows, after PostgreSQL's permissive row security
policies, which are the model. A rows rule grants nothing: the permission a
holder needs is still the role's. A property the caller may lack allows
null, and a comparison with it is false when it is null, so a rule written
`memberId == caller.memberId` reaches no row for a caller with no member
identifier, which is the fail-closed answer. What a caller sees outside
their rows is settled once: a list shows only the rows, a read of one
record by its key answers not found, never forbidden, so that an identifier
does not reveal whether a record exists, and a change that would leave the
rows is refused. Each is a derived case for every operation over the
entity whose permission the role holds, the entity being the one its
answer or request body is, its `guard` or `listOf` names, or, for an
operation that reaches it by its key alone (a delete with no body, an
action on one record), the one its path parameter is the key of, named
`<entity>Id` (`shows only the rows of <role>`,
`a record outside the rows of <role> is not found`, `a change outside the
rows of <role> is refused`), and a job acting as the role gets `acts only
on the rows of <role> in <entity>`. The validator refuses a rule keyed by a
name that is not an entity, one that does not give true or false, and one
naming what is neither the entity's field nor the caller's property
(`rows`); a dot anywhere but in `caller.<property>`, a property the caller
does not declare, and the caller named outside a rows rule
(`caller_property`). The name `caller` is reserved wherever an expression's
names come from: a field, a schema's or a view's property or an algorithm's
input of that name is refused in a file on 0.3 (`reserved_name`), and read
with the version warning in a file on 0.2 (ADR-094). The roles a caller
holds are not a property: a rule is written per role. A test's fixture
names the caller as a role, or as an object with `role` and a value for
each property the test needs.

A tenant scope, every record of an organisation reached only by that
organisation's callers, is said once (ADR-179):

```yaml
tenant:
  property: organisationId   # the caller's property
  key: organisationId        # the field every tenant-owned entity carries
  shared: [Country]          # the entities every tenant reads whole
```

The document as read holds, for every role and every entity with a field
named `key` that `shared` does not list, the rule
`<key> == caller.<property>` under the role's `rows`, at the position of
`tenant`; `specarch model`, the derived cases, the documents and every
generator read it as if written. A rule the role writes for that entity
stands in its place, alone: an organisation's own record, whose identifier
is `id` rather than the key, gets `id == caller.organisationId` on each
role, and a role that sees every tenant's records gets `true`. An entity
that carries no field named `key`, is not listed under `shared` and is not
limited by a rule of its own on every role is refused at the entity, so
that an entity left out is never reached whole; so are a `shared` name
that is no entity, a shared entity that carries the key, a property the
caller does not declare and a key the entity cannot compare with the
property (`tenant`). A scope in a specification with no roles is refused
at `tenant`, since there is no role to derive a rule for. A derived rule
is checked once, at the entity, never at the role.

### Machine clients, callers, a route closed on purpose, and the clients a grant holds through

A program that calls the system is a client (ADR-096, ADR-106, ADR-110),
keyed by its `software_id` of RFC 7591, the publisher's name for the
software across its versions, in kebab-case. An `application` is where a
person signs in; a `service` calls on its own account, with the `roles`
it holds and a `proof` of its identity on each request, or carries a
person's call:

    clients:
      fee-ledger-batch:
        description: The finance system's batch, which reads the day's loans to reconcile the fee ledger.
        kind: service
        roles: [auditor]
        proof:
          kind: signedRequest
          algorithm: hmac-sha256
          covers: ["@method", "@path", content-digest, date]

A client holds roles as a person does, so what it may do is said once,
through permissions, and the fail-closed rule stays one rule. `proof` is
`signedRequest`, a signature over the named parts of the request after
RFC 9421, with the `algorithm` the two sides agree on from the ones the
RFC registers and the `covers` it signs (`@method`, `@path` and the other
derived components, or a header field by its name in lower case), or
`mutualTls`, the client certificate of RFC 8705. The algorithm is design,
not stack, because the client cannot sign without knowing it; the key is
a secret and is never in a specification. A client with roles must have a
proof, and so must a service (`client`); an application without one is a
public client, whose name in a token is the issuer's word and no proof of
origin. `softwareVersion` is the range the client is admitted at, a lower
bound included and an upper bound excluded, each a Semantic Versioning
2.0.0 version, written `>=1.2.0 <2.0.0`, checked once at the edge against
the `softwareVersion` claim of the token the issuer fills from the
registration, so a specification whose tokens declare no such claim is
refused (`software_version`); `softwareStatement: true` says the version
is the publisher's, from the signed statement the issuer verified, which
makes the check a security check rather than a compatibility one. The
`client_id` the authorisation server issues differs per environment, so
it is a deployment value of the implementation file
(`deployments.<environment>.clients.<name>.clientId`), never a design key.

An operation or command says who may call it with `callers`, the
explicit subset: `people`, the reserved word for anyone signed in (which
people is the roles' business), and clients by name:

    get:
      operationId: listLoans
      permission: loans.read
      callers: [people, fee-ledger-batch]

Left out, `callers` is `[people]`: a client is refused. There is no
`all`: a client added later is admitted only once someone writes it on
the route, which is the fail-open the estate declined. The validator
refuses an empty list, a name that is neither `people` nor a declared
client, a client twice, a service with no roles (it only carries a
person's call) and a client none of whose roles grants the permission,
directly or by `implies` (`callers`). A route that exists and is open to
nobody is written with the reserved permission `nobody`, declared under
`permissions` beside `public`: no role is asked to grant it, a role that
grants it and `enabledBy` on such a route are refused, and the route stays
in the OpenAPI document and the techspec, marked closed, so that a client
still calling it gets the problem the design declares rather than a 404
that looks like a wrong path.

A role may narrow a grant to the clients it holds through, keyed by
permission beside `rows`:

    roles:
      invoice-manager:
        permissions: [invoices.read, invoices.delete]
        through:
          invoices.delete: [admin-web]

The check is at every hop against the application the token carries,
`client_id` on an access token and `tctx.application` on a transaction
token, so `through` needs a token that declares one of them; a session
carries none, and a specification with neither is refused (`through`), as
are a permission the role does not grant, a client not declared and an
empty list. Listing nobody is not written: to close the grant, drop the
permission.

The derived cases: `refused for every caller` for a closed route;
`refused for a signed-in person` where `people` is absent and `refused
for every caller not listed` where `callers` is written; `refused with a
bad signature from <client>` for every route a client that signs may
call; `refused through a client not listed for <role>` at every
operation, command and page naming a permission the role narrows;
`refused with <client> at a version outside its admitted range` at the
edge; and, once any service is declared, `refused for a service's own
token` at every operation for people, the confused deputy: a service
calling with its own proof or token in place of the person's, so that the
person, with their rows, roles and `through`, would be lost (ADR-108).

### Tokens, the delegation chain, the audience and security events

A service that issues tokens declares each format under `tokens`
(ADR-097, ADR-108, ADR-109, ADR-111):

    tokens:
      access:
        description: What a signed-in person presents on every call.
        format: jwt
        lifetime: PT15M
        signature: ES256
        claims:
          sub: { description: The person's identifier. }
          aud: { const: https://loans.example.org }
          client_id: { description: The application the person signed in through. }
          memberId: { type: [string, "null"], format: uuid, description: The member signed in; null for staff. }
        default: true
      transaction:
        description: What a hop presents when it calls the next on a person's behalf, inside the trust domain.
        format: jwt
        tokenType: txn_token
        lifetime: PT5M
        signature: ES256
        claims:
          txn: { description: The transaction's identifier, one for the whole call. }
          sub: { description: The person acted for. }
          scope: { description: The permission the edge operation checked. }
          req_wl: { description: The edge workload that requested the token. }
          tctx:
            properties:
              application: { description: The software_id of the application the person signed in through. }

`format` is `jwt` (RFC 7519, with the access-token profile of RFC 9068)
or `opaque`, a reference the service looks up; `lifetime` is how long
one is valid; `signature` names an algorithm of RFC 7518 or RFC 8037
(`ES256`, `RS256`, `EdDSA`, `HS256` and the rest), and `none` is refused,
as RFC 8725 says a verifier must refuse it; the algorithm is design
because the verifier is often another service. `claims` are JSON Schema
properties, camelCase like every property and on the wire as
`info.wireNames` says, except the registered claims of the JSON Web
Token Claims registry that SpecArch knows (`iss`, `sub`, `aud`, `exp`,
`nbf`, `iat`, `jti` of RFC 7519; `act`, `scope`, `client_id`, `may_act`
of RFC 8693; `cnf` of RFC 7800; `txn`, `events`, `toe` of RFC 8417;
`auth_time`, `acr`, `amr`, `azp` of OpenID Connect Core; and, while the
Transaction Tokens draft is a draft, its `req_wl`, `rctx` and `tctx`),
which are written as the registry names them, go on the wire as
registered whatever `info.wireNames` says, and carry the registering
document's type without being retyped; any other underscore name is
refused (`claim`). A claim fills the caller's property of the same name,
null when the token carried has no such claim, and a claim and a caller
property of one name and different types are refused. `checks` are bool
expressions over the claims, each a derived red case wherever the token
is accepted. The one token marked `default` is what every operation
accepts unless it names another with `token`, except an operation only
clients call (`callers` without `people`): it accepts the token the
first listed client with a `token` proof presents, or none when every
listed client proves itself otherwise (a signed request, a certificate),
since the default is the token a person presents. A specification with a
session and no tokens writes none of this.

`tokenType` is `access_token`, the default, or `txn_token`, a transaction
token of the OAuth Transaction Tokens draft: one short-lived JWT minted
once at the edge and passed unmodified through every hop of a call,
carried in the `Txn-Token` header and never in `Authorization`, declaring
`txn`, `sub`, `scope` (the permission the edge checked) and `req_wl` (the
edge workload), no `aud` (its audience is the trust domain), and
`tctx.application`, the application the person signed in through,
immutable from the edge on, which every hop checks `through` against. An
operation accepting a transaction token, or a token declaring `act`,
names `via`, its immediate callers on a person's behalf: services with a
workload-identity proof (`mutualTls`, `spiffe` or `wimse`), each written
out, since a wildcard would admit a hop added later without anyone
writing it; `via` elsewhere is refused. A `token` proof names an access
token a client presents, never a transaction token, since a workload must
not authenticate itself with one.

An access token some operation accepts fixes its audience: `aud` with
`const` naming the service, or `enum` naming the few it stands for, each
a URI when it has a colon (RFC 9068, section 4; RFC 8707; RFC 9700,
section 2.3), so that a token minted for one service is refused at the
next (`token_audience`). A transaction token writes no `aud` and is
refused at a wrong hop by the caller's workload identity instead.

Every refusal of a caller is a security event with a fixed rule and
severity: `token_audience`, `via` and the confused deputy (`deputy`)
high; `callers`, `through` and a version outside the range with a
software statement medium; a self-asserted version and an expired token
low. A specification with tokens and clients declares `securityEvents`,
with `store`, the dependency that is the append-only store, and `alarm`,
the dependency that is the alarm channel an event of severity high, or a
repeat for one client within the window the deployment sets, goes to as
well (`security_events`). An event is a Security Event Token (RFC 8417)
holding the rule, the severity, the operation, the client, the subject
and the hash of the refused token, never the token. Once `securityEvents`
is declared, every derived refusal says which event it writes, so the
generated tests check it.

The derived cases: `denied with expired <token>` on every operation
accepting a token, as a session gives `denied with expired session`;
`<check> fails on <token>` for each check; `refused when the actor is the
subject` for a token declaring `act`; `refused with a token for another
audience` for an access token; `refused with a caller not listed in via`
where `via` is written; and, once a service exists, the confused deputy's
token forms: `refused for a person's token requested by a service with no
actor` on an access token, `refused with a req_wl that is not an edge` and
`refused with a txn differing from the one received` and `refused with a
scope no edge of its chain checked` on a transaction token.

### Signing in at an identity provider

A person signs in at an identity provider through `signIn`, one object
of the design: OpenID Connect's authorisation code flow with PKCE
(ADR-182).

    signIn:
      description: Staff sign in at the organisation's identity provider.
      kind: oidc
      provider: identityProvider
      client: desk-web
      flow: authorizationCode
      pkce: true
      callback: finishSignIn
      identityToken: identity
      logout:
        backChannel: endProviderSession

`provider` is a dependency of kind `identityProvider`; `client` the
client of kind `application` the person signs in through; `flow` is
`authorizationCode` and `pkce` is `true`, with the S256 method of RFC
7636, which RFC 9700, section 2.1.1, asks of every client. `callback` is
the operation of this system the provider sends the person back to: a
GET taking `code` and `state` as required query parameters and naming
the provider under `calls`, since it exchanges the code there; its body,
the exchange, the checks and the session, is the service's.
`identityToken` is the token, under `tokens`, the exchange returns: a
JWT declaring `iss`, `aud` and `nonce`. `logout.backChannel` is the
operation the provider calls when the person signs out there (OpenID
Connect Back-Channel Logout 1.0), which only service clients may call.
Each reference that does not hold is refused at its key (`sign_in`).

The provider's issuer and keys and the client's client_id differ per
environment and per registration, so they are the deployment's, never
the design's: `deployments.<environment>.signIn.issuerUrl`, the issuer
every identity token carries and the base of the discovery document,
`signIn.jwksUri` when the keys are not the discovery document's, and
`clients.<client>.clientId`, the identity token's audience. A `const` or
an `enum` on the identity token's `iss` or `aud` is refused, and so is a
deployment of a specification with `signIn` that gives no `signIn` or no
`clientId` for the client.

A page leaves for the provider with the event `navigate: signIn` (see
Page events). The generated screens send the browser to the
authorisation endpoint the discovery document names, with a new state,
nonce and PKCE verifier; the three go to the callback in the
`__Host-sign-in` cookie on the callback's origin, which the deployment
serves on the page's own origin for the web.

The derived cases, on the callback, each critical and refused with no
session started: `state does not match` (RFC 6749, 10.12; RFC 9700,
4.7), `code used twice` (RFC 6749, 4.1.2; RFC 9700, 4.5), `signature
invalid` (OpenID Connect Core, 3.1.3.7, step 6), `issuer differs` (step
2; RFC 9700, 4.4, the mix-up), `identity token expired` (step 9) and
`nonce replayed` (step 11). Once `securityEvents` is declared each names
its event: `sign_in_state` medium, `sign_in_code`, `sign_in_signature`,
`sign_in_issuer` and `sign_in_nonce` high, `sign_in_expired` low. The
callback derives none of the token cases of an operation a signed-in
caller calls, since no token is issued before it. The techspec shows the
flow as a sequence diagram with the checks and the clause that asks for
each.

### Separation of duties

`separationOfDuties` holds sets of permissions one holder must never have
together, after the static separation of duty of the ANSI RBAC standard
(INCITS 359), which states it as a set and a cardinality: no one holds
that many of the set at once.

    separationOfDuties:
      lend-and-write-off:
        description: Whoever lends a copy must not be able to write the loss off.
        permissions: [loans.create, loans.writeoff]
        cardinality: 2

A set names two or more declared permissions, none of them `public`.
`cardinality` is how many of them one holder may not reach: at least 2, at
most the number the set names, and 2 when left out. A role that grants
`cardinality` or more of a set is refused (`separation_of_duties`). The
validator cannot see one person holding two roles that each grant part of
a set, because who holds which role is data; the techspec lists, for each
set, the combinations of roles that together reach it, as roles never to be
given to one person.

### Sessions

A specification may declare how a signed-in session ends:

    session:
      idleTimeout: PT30M
      absoluteTimeout: PT12H

`idleTimeout` is the time without a request after which the session expires,
`absoluteTimeout` the time after signing in after which it expires whatever
the caller does; at least one is given, and neither is zero (`session`).
The two are the OWASP Session Management Cheat Sheet's, which asks for
both. Once a session is declared, every operation, command and page whose
permission is not `public` gets the derived red case `denied with expired
session`, so that a session that outlives its limit is a test, not a hope.

### Middlewares

The steps a request passes before an operation's handler runs, such as a
session check, a permission check, a tenant, a rate limit or an audit line,
are declared once, each with what it checks or does and the refusals it
answers in place of the handler:

    middlewares:
      require-session:
        description: Refuses a request that carries no session.
        refusals:
          - { status: 401, problem: session-missing }
      audit-log:
        description: Writes one audit line per request.

An operation lists the ones it runs, in the order they run, and `[]` when
it runs none:

    paths:
      /loans:
        get:
          middlewares: [require-session, audit-log]

A name an operation lists must be declared (`middleware`). A refusal's
problem is checked as a response's is (`problem`): under `errors`, of its
own status, and named once the catalogue exists. Once a specification
declares its middlewares, an operation with no `middlewares` is a warning
(`middleware_missing`), held by a must question that blocks the operation
or its middlewares; `[]` says it runs none. Which function of the code each
middleware is belongs to the implementation file, under
`bindings.http.middlewares`, since the design says what is checked and not
how a framework chains it (ADR-131). The technical specification names
each operation's middlewares in its table of operations and in its runtime
view, and lists every middleware with its refusals and the operations that
run it.

### Dependencies

An external system the operations call is declared once, with the time
limit of one call, and an operation names the ones it calls:

    dependencies:
      feeLedger:
        description: The finance system's ledger, where a late fee is posted.
        timeout: PT5S

    post:
      operationId: returnLoan
      calls: [feeLedger]

A `calls` entry must name a declared dependency, and a `timeout` is never
zero (`dependency`). The limit is on the dependency, not on the call site,
so that one system has one limit wherever it is called. An operation that
calls a dependency gets two derived red cases, `dependency fails <name>`
and `dependency times out <name>`, both critical whatever the operation's
harm, because they are the cases nobody exercises by hand; the expected
outcome is the operation's 502, 503 or 504 response when it declares one.

What a dependency is, how it is reached and which dependencies carry a
channel are said with three keys, for the context diagram (ADR-134):

    dependencies:
      eventBus:
        description: The library's message broker.
        timeout: PT1S
        kind: messageBroker
        protocol: amqp

    channels:
      loan.overdue:
        description: Published by the nightly job for each overdue loan.
        carriedBy: [eventBus]

`kind` is one of `system` (another software system with an interface this
one calls), `database` (a store another party keeps that this system reads
or writes directly), `messageBroker` (a broker that carries channels),
`fileStore` (a file exchange or object store) and `identityProvider` (the
issuer of the tokens this system accepts, where `signIn` sends a person);
left out, it is `system`. It
sets the dependency's shape in the context diagram. `protocol` is
AsyncAPI's word: a lowercase URI scheme name (RFC 3986, 3.1), such as
`https`, `grpc`, `amqp`, `kafka`, `postgresql` or `sftp`, kept open as
AsyncAPI keeps it. `carriedBy` names the dependencies a channel lives on
or is delivered to; a channel without it is this system's own, drawn at
its boundary. Which way a message goes is said where it is sent and
received: `emits` sends it, a job's `consumes` trigger receives it. A
`specification` on a dependency of kind `database`, `messageBroker` or
`fileStore` is an error (`dependency`): a specification is a system's.
A `carriedBy` entry that names no declared dependency, or one of kind
`database`, `fileStore` or `identityProvider`, is an error at the entry
(`channel`), and the diagram still draws the channel.

### References between specifications

A module of an estate is specified on its own, and still keeps another
module's key, picks from its list and calls its operations. The other
module is a dependency that names its specification and the version the
references were written against, and the other specification gives itself
that name in `info`:

    # the registry's specarch.yaml
    info: { title: Customer registry, version: 2.4.0, name: customer-registry }

    # the loans module
    dependencies:
      customerRegistry:
        description: The registry of customers, whose numbers the loans keep.
        timeout: PT2S
        specification: customer-registry
        version: 2.4.0

An element of it is then named in `$ref`'s own form, a URI reference: the
dependency's name, `#`, and a pointer into that specification
(ADR-112). It stands where a name of this specification would:

| Where | Form | Example |
|---|---|---|
| a field's `$ref` | `<dependency>#/entities/X`, `#/enums/X`, `#/views/X` or `#/schemas/X` | `customerRegistry#/schemas/Address` |
| a many-to-one or one-to-one relation's `target`, a list page's `entity` | `<dependency>#/entities/X` | `customerRegistry#/entities/Customer` |
| a picker's `source`, a list page's `source`, a `calls` entry | `<dependency>#/paths/<path>/<method>`, the path with each `/` written `~1` | `customerRegistry#/paths/~1customers/get` |

A one-to-many or many-to-many relation cannot lead to another
specification's entity: its `via` would be in the other module's tables. A
page that shows another specification's records is a list, with its entity
and its list of one dependency, and a picker of a relation to another
specification's entity picks from that specification's list.

Given the specification alone, `validate` checks what it can (the
dependency is declared with `specification`, the reference stands where it
may) and counts every reference as unresolved in its summary line, beside
the open questions; `gaps` lists them. That is a gap, not an error: one
team validates its module with no errors. Given the other specification in
the same run (`specarch validate loans/spec registry/spec`), each
reference resolves there and is checked as a local name is: the element
exists, a list lists the entity it is read for, the columns, filters and
picked fields are fields of that entity, and the dependency's `version` is
the other's `info.version`. Each wrong one is the error `reference` at the
entry. A reference to an element a must question blocks there waits on
that question, and `gaps` names it.

Generators read one specification at a time. `generate sql` writes the
column a relation to another specification keeps, with the type its own
`via` field has and no foreign key, since that table is in another
database, and a comment naming the specification and version. A field
typed by another specification's element cannot be typed from this one
alone, so every code target that would write it refuses it as `reference`
at the field; the UI targets refuse a list page or picker that reads
another specification's list for the same reason. The techspec names
each reference with the specification and version it points at.

### Estates

An estate's modules are checked against each other where the estate is,
usually an umbrella repository that holds each module as a submodule. The
estate is the file `specarch-estate.yaml`, of a kind of its own
(`schema/specarch-estate-0.1.schema.json`): it lists each module's
specification by its `info.name`, with the folder that holds its
`specarch.yaml`, relative to the estate file, and the version the estate
holds, and it copies nothing of a module (ADR-113):

    # specarch-estate.yaml
    specarchEstate: "0.1"
    info: { title: Lending estate, version: 1.0.0 }
    modules:
      loans: { location: modules/loans/spec, version: 1.0.0 }
      customer-registry: { location: modules/registry/spec, version: 2.4.0 }

`validate`, `gaps` and `document` given the estate file, or a folder that
holds one, read every module there once, so every reference between them
resolves as when both are named. Each module is checked against its entry,
as the error `module` at the module: its folder holds a specification,
whose `info.name` is the key and whose `info.version` is the version
listed, and no module's operations or jobs call round a cycle of modules
through their dependencies. `gaps` prints the estate's problems before
the modules' open questions documents. `document techspec` also writes
`estate.md`, the module map, into `docs` beside the estate file (or
`--out`): the modules, the version each is held at, and the references
between them as a flowchart. Generators read one module at a time.
`examples/estate` is an estate of the lending desk and a catalogue
service.

### Idempotency

GET, PUT and DELETE are idempotent by RFC 9110 (9.2.2): the same request
twice has the effect of once. A POST or PATCH is not, and a client that
loses the answer cannot safely retry it. An operation makes a retry safe by
naming a header parameter as its idempotency key:

    post:
      operationId: createLoan
      idempotencyKey: Idempotency-Key
      parameters:
        - { name: Idempotency-Key, in: header, required: true, schema: { type: string, format: uuid } }

A request repeated with the same key is answered as the first was and has
no second effect; a different request with a key already used is refused.
The key must be a header parameter of the operation (`in: header`), and the
keyword is refused on a method RFC 9110 already makes idempotent
(`idempotency_key`). The header's name follows the IETF HTTP API working
group's Idempotency-Key draft; the project chooses it. The derived cases
are the golden `repeated with the same <key>` and the red `<key> reused
for another request`.

### Validity

An entity whose records are current for a period names the fields that
bound it:

    Member:
      validity: { from: joinedOn, until: membershipEndsOn }

`until` is the date or instant after which a record is expired; `from`,
which may be left out, the one before which it is not yet valid. Both are
fields of the entity, of one format, `date` or `date-time` (`validity`).
Validity sits on the entity, not on a field, because a record is current or
not as a whole. The derivation uses it where a record is taken as input:
an operation whose body names such a record through a relation of the
entity it returns gets the red cases `expired <field>` and, with `from`,
`not yet valid <field>`.

### Guards

An operation or a command that changes data may carry a guard: the entity
it writes, a precondition over that entity's fields that must hold on each
record as it is at the moment of the change, and the exact number of
records it changes:

    post:
      operationId: returnLoan
      guard: { entity: Loan, precondition: 'status == "open" || status == "overdue"', recordsChanged: 1 }

The checks run with the change itself, so a writer who read a record before
another writer changed it is refused rather than overwriting the other's
change, and a change that would touch more records than expected is refused
as a whole. The entity must exist (`guard`), and the precondition is checked
as a check constraint of the entity is: it parses, names the entity's
fields, and gives true or false (`expression_syntax`, `expression_name`,
`expression_type`). The derived cases are `guard precondition fails`, when
there is a precondition, and `concurrent write`, which is critical whatever
the subject's harm. The guard is also what the guarded operational scripts
of `docs/generators.md` are emitted from.

A transition the entry triggers, a PUT of the entity by one path parameter
and a DELETE by one path parameter say what the change writes. For any
other entry, the guard's `writes` says it (ADR-168): `set`, the fields it
sets, or `delete: true`, that it deletes the records (sets their
soft-delete flag when the entity is deleted softly):

    commands:
      clear notes:
        guard:
          entity: Loan
          precondition: 'status == "returned"'
          recordsChanged: 2
          writes:
            set: { note: "null" }

Each value of `set` is one expression: `given`, the value the operator
gives when running the change, or a literal of the field's type, a string
in double quotes (one of an enum's values), a number, `true`, `false`, or
`null` for a field that is not required or whose type allows null. A value worked out from the
record (`renewals + 1`) is refused, since a record already changed could
not be told from one not yet changed, and a second run would change it
again. A field with no literal (a date, a timestamp) takes `given`. The
rule is `guard`: `writes` on a PUT by its key or a DELETE by its key, which
already say what they write; `delete` beside a transition the entry
triggers; a field the entity lacks, a field of its key or its state field
in `set`, which a transition moves; and a value that is neither `given`
nor a literal of the field's type. A transition the entry triggers still
moves the state, and `set` adds its fields to that change.

### Sensitive data, encryption at rest, audit fields and soft delete

A field says how sensitive its value is, and whether it is stored
encrypted:

    email: { type: string, format: email, sensitivity: personal, atRest: encrypted, lookup: hash }

`sensitivity` is `public`, `internal` (for staff only), `personal` (about a
person: masked in logs, shown only to who may see it) or `credential` (a
password, a token). A response that can carry a credential, through a
`$ref` to its entity or inline, is refused unless the field is `writeOnly`,
JSON Schema's word for a value that is sent and never returned; a personal
field in the response of a public operation is warned about
(`sensitivity_exposed`). Request bodies are not checked, since a credential
arrives in one by design.

`atRest: encrypted` stores the value encrypted, so it cannot be compared in
storage. `lookup: hash` keeps a salted hash beside it, so it can still be a
key, unique, or found by equality. An encrypted field in a primary key or a
unique constraint needs the lookup, and a lookup on a field that is not
encrypted is refused (`at_rest`).

An entity may be `audited: true`: every record carries `createdAt`,
`createdBy`, `lastModifiedAt` and `lastModifiedBy`, set by the system and
never by a caller. With `deletion: soft`, a delete sets a `deleted` flag and
keeps the record; a deleted record is not listed and reads as not found.
The entity does not declare those fields, since the keyword already says
them (`audited`). The derivation gives a list of such an entity the golden
case `deleted <Entity> not listed`, and a read of one by id the red case
`deleted <Entity> read`. How each is done on a stack (the mask rules, the
engine's encryption and key, the column names) is an idiom
(`docs/idioms.md`): pii-in-logs, encrypted-column, audit-fields and
soft-delete.

### Lists, limits and problem types

An operation that answers a page of an entity's records says so, with the
fields a client may search, filter and sort by:

    listOf:
      entity: Loan
      filterable: [status, memberId]
      sortable: [dueOn, loanedAt]
      pageSize: { default: 20, maximum: 100 }

A request outside the lists, or for a page above the maximum, is refused,
never ignored, since an ignored filter answers rows the caller did not ask
for. The fields must be the entity's; an encrypted field is never
searchable or sortable, and filterable only with `lookup: hash`; the default
page fits the maximum (`list_of`). The parameter names and the answer's
envelope are the paginated-list idiom.

`limits` says the largest request body, `maxRequestBytes`, and a `rate`: so
many `requests` `per` a duration, with a `burst` of at least as many
(`limits`).

`errors` is the catalogue of problem types, in the shape of RFC 9457: each,
keyed by a kebab-case name, has its HTTP `status`, a short `title` that
does not change between occurrences, and the `condition` under which it is
answered. A response names its type under `problem`, of its own status.
Once the catalogue exists, every 4xx and 5xx response names one
(`problem`), so a client can branch on the type without parsing a message.

An operation writes what it takes and what it answers. A request body
that is an object naming no property, without `additionalProperties`,
takes something the specification does not write (`request_body_open`),
and responses that are refusals alone, with no 1xx, 2xx or 3xx response
and no `default`, leave the success unwritten (`success_response_missing`).
Both are warnings, held by a must question that blocks the operation, its
body or its responses, as a reader asks it when it cannot read them. A
body that takes any object says `additionalProperties: true` (ADR-130).

### Jobs and menus

Work the system does on its own is a job, under `jobs`:

    markOverdue:
      description: Marks open loans past their due date overdue, and tells the member.
      trigger: { schedule: "0 2 * * *" }
      role: scheduler
      reads: [Loan]
      writes: [Loan]
      emits: [loan.overdue/LoanOverdue]
      retries: { limit: 3, then: deadLetter }
      deadLetter:
        role: librarian
        description: A loan the job could not mark is kept on the list of loans the job could not mark, with the error of its last try.
        steps:
          - { name: Look at it, action: Open the list and read the error., check: Why the loan was not marked is known. }
          - { name: Try it again, action: Try the loan again from the list., check: The loan is marked and leaves the list. }

The `trigger` is exactly one of `schedule`, the five fields of cron in UTC,
`every`, a duration, or `consumes`, a `channel/Message`. The job acts as a
`role`; it may also `calls` dependencies. The role, the entities and the
consumed message must exist (`job`); emitted messages and called
dependencies are checked as an operation's are (`emits`, `dependency`). A
test names a job as its subject with `job: <name>`. A job gets the cases
`runs twice`, critical on its own, the dependency cases for what it calls,
and `an item fails every try` when it retries.

A job whose `retries` say `deadLetter` keeps an item apart after its last
try; its `deadLetter` says what becomes of that item: the `role` whose
people handle it, a `description` of where it is kept and how to find it,
in the design's words, and the `steps` to look at it, then try it again or
discard it, each with a check (ADR-166). The operations guide writes them
under the job, or one sentence naming `deadLetter` when the job has none.
A `deadLetter` role that is not a role, or a `deadLetter` on a job whose
retries do not keep an item apart, is an error (`job`).

`menus` is the navigation, a tree of entries, each a `title` with a `page`
or with `items` of its own. Every page an entry opens must exist (`menu`).
An entry is shown to who may open its page.

### Gateways

A service that registers routes only for its health and its metrics and
forwards every other request, whatever its method and path, to the
service a claim of the bearer token names is a gateway, after RFC 9110's
word, under `gateways` (ADR-141, `docs/gateway.md`):

    api:
      description: Forwards every request no path answers, as received, to the service the token's audience names.
      token: serviceAccess
      claim: aud
      forwards:
        - { value: "https://loans.library.example", dependency: loans }
        - { value: "https://catalogue.library.example", dependency: catalogue }
      calls: [identity]
      middlewares: [audit-log]
      refusals:
        - { status: 401, problem: token-invalid, condition: "The token is missing, badly signed or expired." }

`token` names the token verified, under `tokens`; `claim` one it declares,
read from the verified token only; `forwards` the table, each row a
`value` of the claim and the `dependency`, of kind `system` or
`identityProvider`, a request with it goes to as received. `calls` lists
the dependencies called besides, the identity provider whose keys the
gateway fetches among them; `middlewares` and `refusals` are an
operation's and a middleware's. A gateway names no permission: it decides
nothing, and the service behind it checks every path it serves. Every
request no operation under `paths` answers is forwarded, so a path the
gateway must not forward is an operation that answers it, closed with
`nobody` when nobody may reach it.

A token named, a claim it declares, a non-empty `forwards`, each value
once, a declared dependency of a kind that is an HTTP origin, and no
forward's dependency under `calls` too are checked (`gateway`). When the
claim is `aud`, the forwards are the token's fixed audience: the token is
the services' and not this system's, so it is not `default`, no operation
names it (`gateway`), and its `aud` declares no `const` or `enum`
(`token_audience`); ADR-097's one default is counted among the other
tokens, so a gateway module whose only token is the forwarded one has no
default. `middleware`, `middleware_missing`, `dependency` and `problem`
reach a gateway as they reach an operation, and a server running a
package that implements it is warned of with `link_missing` when no link
leads to a forwarded or called dependency's server. In an estate, a
forward whose dependency names a module given in the run must be one of
the `aud` values that module's access tokens declare (`gateway`), and a
token forwarded by `aud` is left out of the refusal of two modules
accepting one audience.

A test names a gateway as its subject with `gateway: <name>`. A gateway
gets the golden case `forwarded to <dependency>` per forward; `refused
with no token`, `refused with a bad signature` and `denied with expired
<token>`, frequent; `refused with an unknown <claim>`, occasional; for
`aud` or a claim whose type is an array, `refused with a <claim> naming
two forwarded services`, rare; and the dependency cases for every
dependency forwarded to or called, critical.

### Workflows

A request that finishes later, after people approve it, is a workflow,
under `workflows`, keyed by kebab-case name:

    fee-waiver:
      description: A late fee is waived only after a desk supervisor approves it.
      trigger: requestFeeWaiver
      subject: FeeWaiverRequest
      steps:
        - name: approve
          kind: approval
          approvers: [desk-supervisor]
          permission: fees.approve
          deadline: P3D
          onDeadline: refuse
        - name: waive
          kind: operation
          operation: waiveFee

The steps take their meaning from BPMN 2.0, its sequential subset: an
`approval` is a user task whose `approvers` are its potential owners, its
`deadline` a timer on it, and an `operation` step a service task. The
steps run in order, and a refusal at an approval ends the request. The
`trigger` is the operation that starts the workflow; its request body is
the workflow's form, and it answers 202, accepted and not yet done
(RFC 9110). The `subject` is the entity that holds the request while it
waits. An approval names the roles that may approve, the `permission` it
checks, a `deadline`, a duration above zero, and `onDeadline`: `refuse`
ends the request, `escalate` moves it to the later approval named under
`escalateTo`. An operation step names the operation the system calls once
every approval before it has passed.

The person who made a request never approves it. The rule holds on every
approval and has no switch; whether two people are one is known only when
the workflow runs, so it is a derived case, not a check.

The validator refuses a trigger that is not an operation or does not
answer 202, a subject that is not an entity, an approver that is not a
role or does not grant the approval's permission, an approval that checks
the trigger's permission, a role that grants both the trigger's and an
approval's permission while a separation-of-duties set holds the pair,
whatever the set's cardinality, a deadline of zero, an operation step naming no
operation, an escalation to a step that is not a later approval, and two
steps of one name (`workflow`); an approval's permission must be declared
(`permission_undeclared`). A test names a workflow as its subject with
`workflow: <name>`. Parallel approvals, a number of approvals out of a
pool and loops are outside the subset; a workflow that needs one is
written with a question.

### Maker-checker on a page

A form or task page that submits to a workflow's trigger makes the
request, and its submit is answered 202: accepted, and waiting for
approval. The page acts on that answer, a form with its `onSubmitted` and
a task under `onSubmitted."202"`, and the event carries the message the
person reads, which is the page's pending state:

    fee-waiver-form:
      kind: form
      entity: FeeWaiverRequest
      submit: requestFeeWaiver
      onSubmitted: { navigate: loans-list, message: Sent for approval. A desk supervisor answers within three days. }

A list page may be the inbox of an approval step, listing the requests
that wait there:

    fee-waivers-inbox:
      kind: list
      entity: FeeWaiverRequest
      permission: fees.approve
      source: listFeeWaivers
      inbox: { workflow: fee-waiver, step: approve }

The inbox lists the workflow's `subject` and checks the step's
`permission`, so it opens to whoever may approve and to nobody else. A
workflow names the channel messages it publishes when it ends, approved or
refused, under `emits`, as an operation does, so a page waiting on it
knows how the end is announced.

The validator refuses a page submitting to a trigger with no event for
the 202 answer or no message in it, and an inbox on a page that is not a
list, naming no workflow or no approval step of it, listing another entity
than the subject or checking another permission than the step's
(`workflow`); a workflow's message that no channel declares is `emits`. A
form that makes a request gets the derived case `sent for approval`, its
202 answer with the message shown; a task page has it as its case for
the 202 answer. Someone without the step's permission is refused the
inbox by the page's own `denied` case, so it lists nothing to them.

### Page events

A page raises events: a form is submitted, a row of a list is selected,
an operation run from an action succeeds. Each says where it leads:

    member-form:
      kind: form
      submit: createMember
      onSubmitted: { navigate: member-view, with: { memberId: id }, message: The member is registered. }
    members-list:
      kind: list
      onSelect: { navigate: member-view, with: { memberId: id } }
      actions:
        - { label: Remove, kind: operation, target: removeMember, then: { message: The member is removed. } }

`onSubmitted` is a form's, `onSelect` a list's, and `then` an action's of
kind operation; an action of kind navigate leads already. `navigate` names
a page, and `with` gives exactly that page's route parameters, each from
a field of the page's entity: the record submitted, selected or acted on.
A list's action of kind navigate whose target's route takes a parameter
gives it the same way, under the action's own `with`, and is offered on
each row; one whose target takes none is a toolbar button. `with` on
the action of a page that is not a list is refused: a view fills its
actions' routes from its record's fields of their names. `navigate:
signIn` names no page: it leaves the application for the identity
provider's sign-in (see Signing in at an identity provider), from a task
page's answer or an action's `then` only, with no `with` and no
`message`; anywhere else, or with no `signIn` declared, it is refused
(`sign_in`).
`message` is a status message, a full sentence, announced without moving
focus (WCAG 2.2, 4.1.3). A full sentence starts with a capital, a letter
of a script without case or a digit, and ends with a full stop, a
question mark or an exclamation mark, in Latin or CJK form. An event with only a message stays on the page.
Every rule here is `flow`. The concepts are IFML's events and navigation
flows; its diagram notation is not used, and the techspec draws the
screen flow from the pages.

A flow is a task a person does across pages, under `flows`, keyed in
kebab case:

    lend-a-copy:
      description: A librarian finds the member at the desk, opens their record and lends them a copy.
      actor: librarian
      steps:
        - { page: members-list, event: select }
        - { page: member-view, event: action, action: Lend a book }
        - { page: loan-form, event: submitted }

The `actor` is a role, and it must be allowed to open every page on the
way and to take every action a step names. Each step is a page and the `event` on it: `select`, `submitted`, or
`action` with the action's label. Every event but the last must lead to
the next step's page (`flow`). The techspec draws each flow as its steps.
A test names a flow as its subject with `flow: <name>`; its golden case
walks the steps.

### Task pages

A page of kind `task` submits to an operation without loading a record:
sign-in, a second factor, a password reset, a confirmation of an e-mail
address.

    sign-in:
      kind: task
      title: Sign in
      route: /sign-in
      permission: public
      submit: signIn
      fields: [email, password]
      onSubmitted:
        "200": { navigate: home }
        "202": { navigate: second-factor, with: { challengeId: challengeId } }

A task page has no `entity`, `source`, `columns` or `filters`. Its
`fields`, or the fields of its `sections`, are properties of the submit
operation's request body, and every property the body requires is among
them; an operation that takes no body cannot be a task's (`page`). So the
rules a person sees are the body's, written once, in the operation.

`onSubmitted` is keyed by the success status the page acts on, each one
the operation declares (`page`), and each event is a page event as above,
whose `with` takes the route parameters of the page it leads to from
properties of that response's body (`flow`). A problem the operation
answers is shown under the page's failed states, which may name the field
it is about. In a flow, a `submitted` step on a task page leads on when
any of its answers leads to the next step's page.

The derived cases are one per answer: the first success the operation
declares is the golden case, each other success is a golden case
`answered <status>`, and each problem type is a red case `fails with
<problem>`. A public task page has no denied cases and no expired
session. The techspec draws each answer as an edge labelled with its
status.

### Content pages

A page of kind `content` shows copy: headings, paragraphs, images and
links, in the order a person reads them, under its title. It loads and
sends nothing.

    home:
      kind: content
      title: Welcome
      route: /
      permission: public
      blocks:
        - heading: { level: 2, text: Who we are }
        - text: A small studio that builds things.
        - image: { path: images/studio.jpg, alternativeText: "The workshop, with two benches and a lathe." }
        - link: { page: contact, label: Write to us }
        - link: { href: "https://example.org/map", label: Our street on the map }

Each block is exactly one of four:

| Block | Holds |
|---|---|
| `heading` | `level`, 1 to 6, and `text`; the page's title is level 1 |
| `text` | a paragraph |
| `image` | `path`, where the project serves it, relative to the site's root (no scheme, no leading `/` or `\`, no `.` or `..` folder and no `?`, `#`, `%` or space, so it stays on the site), and `alternativeText` |
| `link` | `page`, a page of the specification, or `href`, an address outside it (https, http or mailto only), and `label` |

A content page has no `entity`, `source`, `submit`, `fields`,
`sections`, `columns`, `filters` or `actions` (`page`): `sections` keeps
its one meaning, a form's or a view's fields in titled groups, and a
link to another page is a link block. Its route takes no parameter, and
a link opens a page that exists and whose route takes none, since no
record gives a parameter (`page`, `flow`). Two links of one label lead
to one place: a reader tells links apart by their labels (WCAG 2.2,
2.4.4), and a flow step follows the first link of its label (`page`).
`blocks` on any other kind is refused (`page`). A web generator writes
an image's path from the site's root, so a page on a nested route such
as `/about/team` loads `images/team.jpg` from `/images/team.jpg`.

Every text of a block is a string key of the page, translated as its
title is: a web generator writes the heading's and the paragraph's text,
the image's alternative text and the link's label under
`<page>.blocks.<n>.text`, `.alternativeText` and `.label`, by the block's
place, as a section's title is `<page>.sections.<n>`.

In a flow, a step on a content page takes its event `action` with the
label of a link; it leads where the link's page is. The derived cases
are those of any page: the golden case that it opens, and for a page
that is not public `denied without <permission>` and, once the
specification declares a session, `denied with expired session`. The
techspec draws an edge per link to a page, and the manual says where
each link leads.

### Custom pages

A page of kind `custom` names a screen the project builds by hand, an
editor or a live view that no other kind draws. The generators write its
route, its guard and its menu entry, and the screen is the project's.

    editor:
      kind: custom
      title: Edit the document
      description: The document's text, edited by several writers at once.
      route: /documents/{id}/edit
      entity: Document
      permission: documents.edit
      actions:
        - { label: Done, kind: navigate, target: document }

It has `route`, `title` and `permission` as every page; `entity`, the
records the screen works on, and `description`, what the project builds
there, are optional. Its route may take parameters, which the project's
screen reads. It has no `source`, `submit`, `fields`, `sections`,
`columns`, `compactColumns`, `filters`, `blocks`, `childRows`,
`onSubmitted`, `onSelect`, `pickers`, `fieldConditions`, `checks`,
`enteredTwice`, `states` or `inbox`, each refused at its key: what the
screen loads, sends and shows is the project's. Its `actions` are of
kind `navigate` only, the pages the screen leads to; an action of kind
`operation` is refused (`page`), since the screen calls its operations
itself.

In a flow, a step on a custom page takes its event `action` with the
label of one of its actions. The derived cases are those of any page
that opens: the golden case, and for a page that is not public `denied
without <permission>` and, with a session, `denied with expired
session`; no case for a record that is not found, since what the screen
loads is the project's. Each UI generator imports the screen from a
file the project writes and names in its output, so a missing screen
fails the build or the load and never shows a blank page
(`docs/generators.md`); `generate --check` compares nothing of that
file. The techspec lists the page as the project's own screen with its
description, and the manual says it is built by hand.

### Page states

A page may say what it shows when it is empty or fails:

    loans-list:
      kind: list
      filters: [status, memberId]
      states:
        empty: { message: No loans yet. A loan is made from a member's record. }
        filteredEmpty: { message: No loan matches these filters. }
        failed:
          loan-closed: { message: "This loan was already closed, so nothing changed." }
          default: { message: The loans cannot be shown or changed right now. Try again in a moment. }
    member-form:
      kind: form
      states:
        failed:
          email-taken: { message: Another member already has this email address., field: email }

`empty` is a list's, and `filteredEmpty` a list's with filters. `failed`
is keyed by the problem types the page's operations can answer: the one
it reads or submits, and those its actions run; `default` covers every
problem type not named. A form's failed state may name the `field` it is
about, so the message shows beside it (WCAG 2.2, 3.3.1). Every message
is a full sentence. Loading and submitting have no text: the stack draws
them, and offers to retry a failed read and to clear filters the same way
on every page.

A page without `states` shows what its stack shows. Once a page has
`states`, they are complete: a list has `empty`, a list with filters has
`filteredEmpty`, every problem type has a message or there is a
`default`, and nothing else is named (`state`). Each state is a derived
case of the page: `empty`, `filtered empty`, and `fails with <problem>`.

### Sections

A form or a view gives its fields once: as `fields`, or as `sections`,
each a `title` and its `fields`, in the order a person reads them, which
is the focus order (WCAG 2.2, 2.4.3):

    member-view:
      kind: view
      sections:
        - { title: Member, fields: [cardNumber, fullName, email, tier] }
        - { title: Membership, fields: [joinedOn, membershipEndsOn, outstandingFees] }

A page with neither or both, a field in two sections, and sections on a
list are refused (`page`). Where the sections sit on a screen is the
stack's.

### Compact screens

A list says which of its columns a compact screen keeps, in order:

    loans-list:
      columns: [memberId, bookId, loanedAt, dueOn, status, lateFee]
      compactColumns: [memberId, dueOn, status]

Each must be one of the list's columns, and only a list has them
(`page`). How small compact is, and how a compact row is laid out, are the
stack's.

### Child rows

A form edits the records of a one-to-many relation of its entity as rows
under it, each with a `title`, the `fields` of the relation's target shown
in a row, the `maximum` number of rows, loaded and new together, and
`lockLoadedRows: true` when the rows loaded with the record cannot be
changed or removed, so only new ones change:

    member-loans:
      kind: form
      entity: Member
      childRows:
        - { relation: loans, title: Loans, fields: [bookId, dueOn], maximum: 6, lockLoadedRows: true }

Each row is validated by the target entity's own schema. Child rows on a
page that is not a form, a relation named twice, and a relation the
form's entity lacks or that is not one-to-many are refused (`page`); a
field the target lacks is refused (`field`). A many-to-one or one-to-one
relation holds one record, not rows, and the rows of a many-to-many
relation are records of its join entity. A maximum gives the page the
derived case `<relation> with more than <maximum> rows`. Whether rows are
edited inline or in a dialog is the stack's.

### Page elements

A form picks a field that holds another record's key from a list of
those records, says when a field is read-only or hidden, checks rules
across its fields, and asks for a field twice; a task checks and asks
twice as a form does; an action is offered only in some states and may
ask for a reason:

    loan-form:
      kind: form
      entity: Loan
      fields: [memberId, bookId, lentOn, dueOn, pin]
      pickers:
        memberId: { source: listMembers, shows: [cardNumber, fullName] }
      fieldConditions:
        dueOn: { readOnlyWhen: 'lentOn == dueOn' }
      checks:
        due-after-lent: { expression: dueOn > lentOn, message: A copy is due after the day it is lent., field: dueOn }
      enteredTwice: [pin]
    members-list:
      kind: list
      actions:
        - { label: Deactivate, kind: operation, target: deactivateMember, when: 'status == "active"', confirm: "Deactivate this member?", reason: reason }

- `pickers` are a form's, keyed by a field it shows. The record's entity
  is the target of the many-to-one relation of the page's entity whose
  `via` is the field; `source` is an operation whose `listOf` names that
  target; `shows` are the target's fields a person reads to choose;
  `fills` sets other fields of the form from the chosen record, each
  from a field of the same type. Every role that may open the form may
  also call the source (`picker`).
- `when` on an action is an expression over the record it acts on: a
  row of a list, or the record a view or an edit form shows. A form
  without `source` creates a record, so its actions have no `when`.
- `reason` on an action that runs an operation and has `confirm` names
  the property of the operation's request body that carries the reason
  typed in the confirmation, a string the body requires (`action`).
- `fieldConditions` are a form's or a view's, keyed by a field it shows:
  `readOnly`, `readOnlyWhen` and `hiddenWhen`. Every field of a view is
  read-only already. A page is one mode: a form without `source`
  creates, a form with one edits, and a view shows; a field hidden in a
  mode is one that page does not list.
- `checks` are a form's or a task's, keyed in kebab case, each an
  `expression` over the fields it shows, a `message` in a full sentence
  and the `field` it shows beside. `enteredTwice` names fields a person
  types twice; the second entry is compared and never sent. On a task
  the fields are the properties of the request body it sends, so a
  confirmation the operation takes is checked against the password
  before it is sent:

      reset-password:
        kind: task
        submit: resetPassword
        fields: [code, newPassword, confirmPassword]
        checks:
          confirmation-matches: { expression: confirmPassword == newPassword, message: The two passwords are not the same., field: confirmPassword }

Each expression is in the subset under Expressions and gives true or
false, never null. On a form that creates a record it names the fields
the form shows, on a task the properties of its request body it shows,
and elsewhere the entity's fields. The misuses are
`picker`, `action` and `form_field`; an expression that does not parse,
names what it cannot or has the wrong type is an expression error. The
techspec lists every element of every page.

### Accessibility

The specification names its target once, in the design:

    accessibility: { standard: WCAG 2.2, level: AA }

With it, every field a page shows or filters by has a `title`, JSON
Schema's keyword, which is the label a person reads beside it (WCAG 2.2,
3.3.2 and 2.4.6), and no two actions of a page share a label (4.1.2);
both are `accessibility`. Without it, neither is checked; the theme's
contrast is checked either way. The techspec lists the criteria of the
level that the design settles or leaves to the generator, and says who
meets each; every other criterion is checked at commissioning.

### Theme

The visual design is one object, `theme`, in `design/theme.yaml`. Its
`tokens` are in the format of the W3C Design Tokens Community Group:
groups and tokens, each token a `$value` and a `$type` given by it or a
group above it, an alias of another token written `{group.token}`, which
takes that token's type when nothing else gives one, and the format's
`$description`, `$extensions` and `$deprecated`, which SpecArch keeps and
does not read:

    theme:
      tokens:
        color:
          $type: color
          text: { $value: { colorSpace: srgb, components: [0.1059, 0.1216, 0.1412], hex: "#1b1f24" } }
          background: { $value: { colorSpace: srgb, components: [1, 1, 1], hex: "#ffffff" } }
          accent: { $value: { colorSpace: srgb, components: [0.0431, 0.3608, 0.6784], hex: "#0b5cad" } }
          link: { $value: "{color.accent}" }
        space:
          $type: dimension
          small: { $value: { value: 8, unit: px } }
      modes:
        dark:
          color.text: { colorSpace: srgb, components: [0.902, 0.9098, 0.9216], hex: "#e6e8eb" }
          color.background: { colorSpace: srgb, components: [0.0627, 0.0706, 0.0784], hex: "#101214" }
          color.accent: { colorSpace: srgb, components: [0.4235, 0.6902, 0.9608], hex: "#6cb0f5" }
      pairs:
        - { text: color.text, background: color.background, use: text }
        - { text: color.link, background: color.background, use: text }

The types taken are color (in the srgb space, three components from 0 to
1, and an `alpha` and a `hex` that must be the components' colour),
dimension (px or rem), fontFamily, fontWeight, duration (ms or s) and
number. A number is written in decimals: a hexadecimal, octal or binary
integer and a digit separator are refused. `modes` gives tokens, by path, another value; the tokens' own
values are the default. Each pair is a `text` colour on a `background`
colour, with its `use`: `text`, `largeText` (18 point, or 14 point bold)
or `control` (the parts of a control and graphics). Its contrast must
reach what WCAG 2.2 asks of the use, at AAA when the accessibility target
is AAA and at AA otherwise, since WCAG asks no contrast at level A, in
every mode: 4.5:1 for text, 3:1 for large text
and controls, and at AAA 7:1 and 4.5:1 for text (1.4.3, 1.4.6, 1.4.11).
A translucent colour in a pair is refused, since its contrast depends on
what lies beneath. Every rule here is `theme`. How a token becomes code is
the stack's.

### Views

A list page shows a row of one entity with names and counts joined onto
it. That row is a view, under `views`, named like an entity:

    LoanRow:
      description: A loan as the desk's list shows it.
      from: Loan
      properties:
        memberName: { path: member.fullName }
        branchName: { path: member.branch.name }
    MemberRow:
      from: Member
      properties:
        openLoans: { count: loans }
    MemberWithLoans:
      from: Member
      properties:
        loans: { rows: loans }

A view holds one row per record of its `from` entity and carries every
field of it, the audit and deleted fields included, so its key still
names one record. Each property it adds is exactly one of `path`,
relations separated by dots that each lead to one record (many-to-one or
one-to-one) and end in a field; `count`, a relation of the entity that
leads to many (one-to-many or many-to-many); or `rows`, a one-to-many
relation of the entity whose records the view carries. A path has its
field's type and is null when a relation on it has no record; a count is a
64-bit integer and leaves out softly deleted records; rows are a list of
the relation's target, empty when it has none, softly deleted records
left out. The rows of a many-to-many relation are records of its join
entity, so they are carried through the entity's one-to-many relation to
the join entity. A property may not repeat a field of the entity, and a
view may not share an entity's name (`view`).

Rows are how a record is read with the records of a relation, as a form
that edits them as child rows loads it. A list holds one row per record,
so a view with rows is never a list's subject (`list_of`), and rows are no
column in SQL: a SQL view holds one row of plain columns per record, the
rows are the related table's own, read through the relation's key, and a
view that adds only rows has no SQL view at all.

A view is never written. It may be the item of a response, as
`$ref: "#/views/LoanRow"`, and the subject of a list, as
`listOf: { view: LoanRow, ... }`, whose whitelists then name the view's
fields; under a request body it is refused (`view`). The SQL standard lets
a view be updated only when it reads one table without grouping, and the
engines differ beyond that, so a write goes to the entity, its one place.
In SQL a view is named as a table is: the view's name in snake case,
unless the implementation file's mapping names it (`target: view
loan_rows`).

### Value objects

Data passed around but not stored, with no identity of its own (a
diagnostic, a request's summary, a token's claims), is a schema under
`schemas`, named as OpenAPI names the shapes under `components.schemas`:

    Diagnostic:
      description: One thing a check found, passed back and never kept.
      type: object
      properties:
        severity: { $ref: "#/enums/Severity" }
        text: { type: string, maxLength: 200 }
      required: [severity, text]

A schema has `type: object`, `properties` and `required`, and nothing only
a stored record has: no primary key, relations, constraints, states,
validity, audit or deletion. A request body, a response, a message's
payload and another schema's property refer to it as
`$ref: "#/schemas/Diagnostic"`, and a derived case reads a request body's
required properties through it as it does through an entity. An entity
may not relate to a schema, since a relation leads to a record and a
schema has none; a schema may not share the name of an entity or a view,
since all three become schemas of one interface (`value_object`).
`generate sql` writes no table for a schema; `generate openapi` writes it
under `components.schemas` as it is.

An entity's field may hold a schema, or a list of them, as a part of the
entity's record (ADR-063):

    Member:
      properties:
        address: { $ref: "#/schemas/Address" }
        preferences: { $ref: "#/schemas/Preferences", storage: json }
        phones: { type: array, items: { $ref: "#/schemas/Phone" } }

`storage` says how the value is kept in the entity's row. `columns`, the
default for one value, gives each part a column named after the path to
it: `address_street`, `address_location_latitude` for a schema inside the
address. A part's column is NOT NULL only where the part and every value
above it are required, and an optional value gets a check that it is
either wholly absent or has every required part. `json` keeps the whole
value in one column of the dialect's JSON type, and is the only storage
for a list: a list that must be queried row by row has an identity, so it
is an entity related to its owner. In columns a value may not hold a list
of schemas, itself at any depth, or a reference to an entity, and an
optional one needs a required part that is never null, or a row without
it could not be told from one where every part is empty; `storage` goes only on an
entity's own field that holds a schema. Each is refused as
`value_object`. In the interface the field is a reference to the schema
whatever its storage.

A specification read from sources finds the value again (ADR-077). `extract
openapi` writes an entity's property that refers to a component schema as a
reference to it, with `storage: json` where columns would be refused, and
`extract database` writes a JSON column as a field of type `object` with a
question asking which schema it holds. `specarch merge` writes each field
holding a schema out as `generate sql` does and reads the code side's
columns back as that field where they are exactly what it writes: every
part's column, with its type, width and nullability, and the check that an
optional value is wholly absent or has its required parts. Where they
differ it asks at the field, and where no tree names a field but an
entity's columns are every part of a schema under one prefix, it asks
whether they are one value.

### Secrets

A setting under `configuration` says whether it is a `secret`. A secret's
value is never written anywhere in a specification or an implementation
file: not as a `default`, not under a deployment's `configuration`. The
validator refuses both (`secret_value`). The setting's description says
where the value comes from, such as the host's secret store.

### Settings that switch an element on

An operation, a command or a page served only while a setting is on names
it under `enabledBy`. The setting is a boolean of `configuration`; the
validator refuses a name that is not a setting, or a setting of another
type (`setting`), since a switch has two positions and anything else would
need a comparison written somewhere. With the setting off the element is
refused as not available, and the derived case `disabled by <setting>`
asks for a test of it.

    paths:
      /exports:
        post:
          operationId: startExport
          enabledBy: exportsOn

### Expressions

`check` constraints and `formula` strings are written in a small subset of
CEL, the Common Expression Language (cel.dev). Its syntax is the one C, Java
and JavaScript use, so most readers already know it, and its type rules are
strict: values of different types never mix without a written conversion.
CEL is strict on purpose. An implicit conversion is where precision is lost
without anyone seeing it (an int64 turned into a double, a decimal into a
float), so every conversion is written where it happens.

A check is one expression that gives a bool, and so is the `where` of a
unique constraint: the fields must be unique only among the records it
holds for; a role's `rows` rule is one that gives a bool over an entity's
fields and the caller's properties. A formula is one expression
whose type is the algorithm's output type. The validator parses every
expression, refuses anything outside the subset by name, checks every name
and type against the declared fields and inputs, and evaluates every formula
on its worked examples.

Types in expressions are CEL's: `int` (every integer field, int32 included,
is an int; the field's width is checked when a value is computed), `uint`,
`double`, `string`, `bool`, `bytes`, `timestamp` and `duration`, plus three
that CEL does not have and SpecArch adds: `decimal`, `date` and enum values.
CEL has no decimal because it was built for protocol buffers, which have
none; money needs one.

| Part | Example | Meaning |
|---|---|---|
| int | `21`, `-3` | a 64-bit integer |
| uint | `21u` | an unsigned 64-bit integer |
| double | `0.5`, `2.0` | a 64-bit float; `2` is an int and `2.0` a double |
| string | `"open"`, `'open'` | text, in double or single quotes |
| bool | `true`, `false` | |
| `null` | `returnedAt == null` | no value; only for a field that allows null |
| name | `dueOn` | a field of the entity (in a check or a rows rule) or an input of the algorithm (in a formula) |
| `caller.<property>` | `memberId == caller.memberId` | a property of the caller, in a rows rule only; the one place a `.` may appear |
| `( )` | `(a + b) * c` | grouping |
| `-x` | `-discount` | negation |
| `!x` | `!active` | not |
| `*`, `/`, `+`, `-` | `subtotal + tax` | arithmetic on two values of the same numeric type |
| `==`, `!=` | `status == "open"` | equal, not equal; same type on both sides |
| `<`, `<=`, `>`, `>=` | `copiesAvailable <= copiesOwned` | order; same type on both sides |
| `&&`, `\|\|` | `returnedAt == null \|\| returnedAt >= loanedAt` | and, or |
| `c ? a : b` | `daysLate > 0 ? 1 : 0` | `a` when `c` holds, otherwise `b`; both of one type |
| `int(x)` | `int(round(total, 0))` | to int, from uint, a string of digits, or a rounded decimal or double |
| `uint(x)` | `uint(count)` | to uint, from int or a string of digits |
| `double(x)` | `double(count)` | to double, from int, uint or decimal; the result is approximate |
| `decimal(x, scale)` | `decimal(daysLate, 0)`, `decimal("0.50", 2)` | to decimal with that scale, from int, uint, a quoted number, or a decimal of no larger scale |
| `string(x)` | `string(count)` | to string |
| `date(x)` | `dueOn > date(loanedAt)` | the date of a timestamp, or a quoted date: `date("2026-10-07")` |
| `timestamp(x)` | `timestamp("2026-10-07T09:30:00Z")` | a quoted RFC 3339 instant |
| `duration(x)` | `duration("P14D")`, `duration("PT2H")` | a quoted ISO 8601 duration in days, hours, minutes and seconds |
| date `+`, `-` duration | `dueOn == loanedOn + duration("P14D")` | a date moved by whole days, the date first and the days written in place |
| `size(x)` | `size(fullName) <= 200` | the length of a string or a list, an int |
| `round(x, places)` | `round(total, 2)` | a decimal or double rounded half away from zero; a decimal result has that scale |
| `floor(x)`, `ceil(x)` | `floor(hours)` | down or up to a whole number, keeping the type |
| `min(a, b, ...)`, `max(a, b, ...)` | `min(fee, replacementCost)` | the smallest or largest of two or more values of one type |

A date moves only by whole days written in place: `loanedOn + 14` is
refused, since a number names no unit, and so is `duration("PT12H")`
added to a date, since a date has no time of day. `duration` reads the
ISO 8601 form every other duration of the meta-model is written in, not
CEL's `"336h"`, which has no day because CEL has no date.

Decimal scales are tracked. `+` and `-` give the larger of the two scales,
`*` the sum of the scales, `min`, `max` and `? :` the larger; `/` gives an
exact result of no fixed scale, so it has to be rounded before it is
returned. A formula's decimal result must have no more places than the
output's `scale`: write `round(x, 2)` where the design wants rounding. A
conversion that would lose information is refused unless the rounding is
written: `int(round(x, 0))`, `decimal(round(x, 2), 2)`. A double cannot be
turned into a decimal at all, because the digits it lost cannot be brought
back.

Nothing else is in the subset: no `%`, no `in`, no lists or maps written in
the expression, no field access with `.`, no macros (`has`, `all`,
`exists`, `map`, `filter`), and no other function. Each is refused with a
message naming it. Comparisons do not chain: `a < b < c` is an error,
written `a < b && b < c`. CEL itself lets numbers of different types be
compared; SpecArch does not, so that every operator follows one rule, and a
mixed comparison is written with a conversion. Anything a real
specification needs beyond this list is added on purpose, with an example
here.

A worked example's values have the declared types: an int32 or int64 is a
YAML integer, an int64 carried as text a quoted string of digits, a decimal
a quoted string (`"0.50"`) with no more places than its scale, a date a
quoted date, a timestamp a quoted RFC 3339 instant.

### Tests

Tests are part of the specification, split along the same line as
everything else. The specification's `tests` say what must hold on every
stack; an implementation file's `testing` says how one stack runs them, plus
the tests of its own code (unit and integration suites, fixtures, mocks,
performance targets, platforms).

A design test is about one subject: an `operation`, a `command`, a `page`,
a `job`, a `gateway`, a `flow`, a `workflow`, a `requirement`, or an `entity`, with one of its `constraint`s or
`transition`s or alone for its state machine. It has a
`level`, `system` (the test exercises the system through its interfaces as a
client would) or `acceptance` (it shows a stakeholder that a requirement is
met, and names it under `verifies`); unit and integration tests belong to
the implementation's suites, which carry the same `level` key. It is marked
`scenario: golden` for the path that succeeds or `scenario: red` for a path
that fails, and says what happens in three plain sentences: `given`, `when`
and `then`. For a generator a test may add `fixture` (the caller and the
records that exist), `input` (what the call carries) and `expect` (the
status, exit, body, state and messages after), in the design's own
vocabulary and checked against it (`test_data`; `docs/test-generation.md`
gives the shapes). There is no test language beyond that. In a tree, each
test is `tests/<name>/test.yaml`, with the scenario's own data files beside
it, for what the structured keys cannot hold; a test says one thing one
way, so a test with `input` has no `input/` folder and one with `expect`
no `expected/` folder:

    tests/lend-limit-reached/test.yaml
    tests/lend-limit-reached/input/request.json
    tests/lend-limit-reached/expected/response.json

    operation: createLoan
    level: acceptance
    scenario: red
    covers: [response 409]
    given: a standard-tier member with three open loans
    when: createLoan is called for a fourth book
    then: it answers 409 and no loan is created
    verifies: [LIB-3]

Every operation, command, page, constraint and transition needs at least
one golden scenario and its red ones; any golden test of it is its success
case, and the test the validator suggests when there is none carries the
outcome the design gives (the 2xx response, exit 0, the page shown, the
state reached). A requirement and a state machine are subjects of a
different kind: every case is golden and asked for on its own.

A suggested test is named after its subject, then its case:
`create-loan-missing-member-id`. The subject's name is made a valid test
name first, kebab-case with a letter first: requirement `AB-1.2` gives
`ab-1-2-acceptance-1`, every run of characters other than small letters
and digits becoming one hyphen, and a name that starts with no letter
takes its subject key in front. Where two subjects of one key have
different names that give the same valid name, as `AB-1.2` and `AB-1_2`
do, each after the first in document order takes a suffix, `-2`, `-3`.
When subjects of different kinds give
the same name, as page `sign-in` and operation `signIn` both give
`sign-in`, each takes its subject key in front: `page-sign-in-succeeds`
and `operation-sign-in-succeeds`.

- A `requirement` subject has one case per acceptance criterion,
  `acceptance 1`, `acceptance 2` and so on, with the criterion as its
  outcome; its tests are usually `level: acceptance`.
- An `entity` alone is its state machine, declared by `stateField` and
  `transitions`. Its cases are its paths from an initial state (one no
  transition reaches) to a terminal state (one no transition leaves),
  taking the transitions in document order and never visiting a state
  twice, named by their states: `open to overdue to returned`.

The validator derives the cases a subject needs from the rest of the
specification. Each case has a frequency, how often users make that
mistake:

| Found in the design | Case | Scenario | Frequency |
|---|---|---|---|
| a required field of a request body | `missing <field>` | red | frequent |
| a required part of a value object a body field holds, at any depth, or of each item of a list of them | `missing <field>.<part>` | red | frequent |
| `minimum` / `maximum` of a field or parameter | `<field> below minimum N` / `<field> above maximum N` | red | occasional |
| | `<field> at minimum N` / `<field> at maximum N` | golden | occasional |
| `exclusiveMinimum` / `exclusiveMaximum` | `<field> at exclusive minimum N` / `... maximum N` | red | occasional |
| `minLength` / `maxLength` | `<field> shorter than N characters` / `<field> longer than N characters` | red | occasional |
| | `<field> of N characters` | golden | occasional |
| `minItems` / `maxItems` | `<field> with fewer than N items` / `<field> with more than N items` | red | occasional |
| `pattern` | `<field> not matching its pattern` | red | frequent |
| an enum | `<field> not one of its values` | red | frequent |
| a format that values can break (email, uuid, date, decimal...) | `<field> not a valid <format>` | red | frequent |
| a permission other than `public` on an operation, command or page | `denied without <permission>` | red | frequent |
| a path parameter, or a route parameter of a page | `not found <parameter>` | red | frequent |
| a body field that is the `via` of a relation of the entity the operation returns | `not found <field>` | red | frequent |
| a unique constraint of the entity a POST with a 201 response creates | `duplicate <constraint>` | red | occasional |
| a channel the operation `emits` on | `dependency fails <channel>` | red | rare |
| a dependency the operation `calls` | `dependency fails <dependency>`, `dependency times out <dependency>` | red | rare |
| an `idempotencyKey` | `repeated with the same <key>` | golden | frequent |
| | `<key> reused for another request` | red | occasional |
| a `handshake` on an operation | `expired handshake <name>` | red | frequent |
| a body field that is the `via` of a relation to an entity with `validity` | `expired <field>`, and with `from` `not yet valid <field>` | red | occasional |
| a GET answering a list of an entity with `deletion: soft` | `deleted <Entity> not listed` | golden | occasional |
| a GET by a path parameter answering an entity with `deletion: soft` | `deleted <Entity> read` | red | occasional |
| an operation with `listOf` | `page beyond last` | golden | occasional |
| an operation with `listOf` | `page size above <maximum>`; with `sortable`, `sort by a field not sortable`; with `filterable`, `filter by a field not filterable` | red | occasional |
| an operation with `limits.maxRequestBytes` | `request larger than <n> bytes` | red | occasional |
| an operation with `limits.rate` | `rate exceeded` | red | occasional |
| a job | `runs twice` | golden | critical on its own |
| a job that calls a dependency | `dependency fails <name>`, `dependency times out <name>` | red | critical on its own |
| a job with `retries` | `an item fails every try` | red | occasional |
| an approval of a workflow | `refused at <step>`, `approval without <permission>` | red | frequent |
| | `deadline passes at <step>` | red | critical on its own |
| a workflow with an approval | `requester approves own request` | red | critical on its own |
| a `session`, for every permission other than `public` | `denied with expired session` | red | frequent |
| a `guard` with a precondition | `guard precondition fails` | red | occasional |
| a `guard` | `concurrent write` | red | rare |
| a 4xx or 5xx response | `response <status>` | red | occasional |
| a command | `usage error`, and `exit <status>` for each non-zero exit code | red | frequent |
| a check constraint / a unique constraint | `violates <constraint>` / `duplicate <constraint>` | red | occasional |
| a unique constraint with `where` | `duplicate outside the condition` | golden | occasional |
| an operation, command or page with `enabledBy` | `disabled by <setting>` | red | frequent |
| a check constraint that can be false in more than one way | `violates <constraint>: <clause> is false`, one per way (below) | red | occasional |
| an acceptance criterion of a requirement | `acceptance <N>` | golden | occasional |
| a path of a state machine | `<state> to <state> to ...` | golden | occasional |
| a transition | `from wrong state` | red | frequent |
| a picker of a form | `picker <field> finds nothing` | red | occasional |
| an action with `when` | `<label> not offered` | red | occasional |
| an action with `reason` | `<label> without a reason` | red | frequent |
| a check of a form | `violates <check>`, or one per way it can be false | red | frequent |
| a field entered twice | `<field> entered twice differently` | red | frequent |

A check constraint whose expression joins clauses with `&&` or `||` is a
decision table (ISO/IEC/IEEE 29119-4, 5.2.6): one red case per way it can
be false, each clause of an `&&` false alone and every disjunct of an `||`
false at once. The case names the false clauses in the expression's own
text, `violates copies_in_range: copies >= 0 is false`, so that every
build names it alike; a constraint false in only one way keeps the plain
`violates <constraint>`. A case name holding a colon or a quote is quoted
in `covers`.

An acceptance case is chosen when its requirement names a `harm`, and a
path when one of its transitions satisfies a requirement with a harm; the
others are left out, as below.

A field or a parameter's schema may replace the frequency of every case
derived from it (its limits, pattern, enum and format, and `missing` or
`not found` for it) with `mistakes`:

    fullName: { type: string, maxLength: 200, mistakes: rare }
    email: { type: string, format: email, mistakes: frequent }

A requirement may name what is at stake when it is not met, under `harm`:
`data-loss`, `money`, `security`, `safety`, `privacy` or `availability`.
`priority` does not do this: it says whether a release may go without the
requirement, not what its failure costs.

    LIB-3:
      statement: A member shall have at most three open loans.
      harm: [money]

A requirement may also name the release it is meant for, under `release`,
by version. A release record of that version must exist, planned or
released (`record_ref`), so a scope split into a first and a later release
is written in the requirements, and the test plan groups the requirements
by release with the tests that verify each.

Each derived case then has a rank. It is `critical` when its subject
satisfies, under its own `satisfies`, a requirement with a harm, or when
it is a case nobody exercises by hand (a failing or slow dependency, two
writers on one record); `frequent` when its frequency is frequent; and
`other` otherwise. The validator warns once for every critical or frequent
case no test lists under `covers` (`test_case_missing`), with a test to
copy. The cases of rank `other` that no test covers are left out: the test
plan lists them under "Derived cases left out", each with the reason, and
writing a test for one removes it from the list. Once a requirement names
a harm, the traceability matrices have a Harm column.

One test may cover several cases when they are one scenario (a lookup that
answers 404 covers `not found memberId` and `response 404`). A case that
does not apply is covered by a test with `notApplicable` and a written
reason instead of given, when and then. In a file that says
`specarch: "0.2"` or `"0.3"` a test `specarch derive` would write and nobody
has is an error: a chosen case no test covers (`test_case_missing`), and a subject
with no golden test when the design gives its success
(`test_golden_missing`). A subject an open must or should question holds
up keeps a warning, since derive leaves its tests out too until the
question is answered, and so does a subject with no red scenario and no
derived red case (`test_red_missing`), which has no case to write. Derive
runs past these errors, since they are what it settles, and `specarch
extract` and `specarch merge` run it into every tree they write. A file
that says `specarch: "0.1"` is read until 1.0: its missing tests are
warnings, and it gets one warning at the `specarch` line
(`meta_model_version`) until it is moved on (ADR-055); so is one that says
`"0.2"`, whose one warning says that 0.3 reserves the name `caller`
(ADR-094).

An implementation file's suite names the design tests it runs, by name
(`designTests`) or by subject (`designTestsOf: [{ command: validate }]`),
or says `implementationOnly: true`, and carries its `level`: a suite that
runs design tests is `system` or `acceptance`, an implementation-only suite
`unit` or `integration`. The validator checks every name it gives exists.

### Commissioning records

A commissioning run's results are records, not design: a fact about one
run. They live outside the specification, one file per run, under
`records/commissioning/` beside the specification's folder, named by date
and environment (`records/commissioning/2026-10-07-production.yaml`). Each
holds the `environment`, the `date`, the `version` of the specification and
the build, the `operator` as a role, `results` keyed by check name with
`result` (pass, fail or skipped) and a `note`, and the `signoff` with who
signed and when. The commissioning procedure (`specarch document
commissioning`) is the form a run fills in. A commissioning record is one
kind of record; `docs/maintenance.md` describes them all and the rules the
validator checks them against.

### Approval records

`specarch approve --by <stakeholder>` records that a stakeholder read the
documents of a specification and approves it for code generation, as
`records/approvals/<version>.yaml` beside the specification's folder:

    specarchRecord: "0.1"
    kind: approval
    version: 1.4.0
    approvedBy: product-owner
    date: "2026-10-08"
    documents: [techspec, requirements, testplan]
    digest: sha256:9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08

The record is written only when the specification has no error and no
`must` or `should` question, `--by` is one of its stakeholders, and every
configured document on disk is what the specification generates now. The
digest is the SHA-256 of every `.yaml` file under the specification's
folder, in byte order of their paths relative to it, each as its path, a
zero byte, its bytes and a zero byte; any later change to one of those
files voids the approval. `specarch generate` refuses a target while a
`must` or `should` question blocks a section it reads (the sections the
implementation file names under `targets.<name>.reads`, or every section),
and refuses without an approval whose digest is the digest of the files
now; `--unapproved` lets it through both and writes a draft that says so
in every file (ADR-086). An earlier approval of the same version is
replaced.

### What the schema cannot check

JSON Schema validates shape: types, required keys, patterns, closed objects.
It does not know that `target: Loan` must name an entity of the
specification, that a `primaryKey` field must exist in `properties`, that a
page's `columns` belong to its `entity`, that a `transition` uses values of
the state field's enum, that a citation's `source` is declared, or that a
check's `environment` exists. Those are cross-reference checks, made by
`specarch validate` together with the layout, expression, worked-example,
access and traceability checks. A tree that passes the schema can still
fail the validator.

## Markdown sections

The `specarch.md` file beside the root file explains the specification. The
sections follow arc42, trimmed to what a small system needs. Each heading is
fixed so a target can find it and so readers of different specifications
know where to look.

1. Introduction and goals: what the system is for, who uses it, the top
   quality goals.
2. Constraints: technical, organisational, legal; the `constraints` and
   `assumptions` of the requirements stage, explained.
3. Context: the system and its neighbours. The generated context diagram,
   once a dependency has a `kind` or a `protocol` or a channel a
   `carriedBy`.
4. Solution strategy: the few decisions that shape everything else, each
   pointing at its ADR.
5. Building blocks: generated entity diagram, then prose on each entity
   group.
6. Runtime view: for each operation, generated tables of its parameters,
   its request body and its responses, and a generated sequence diagram;
   generated state diagram for each entity with a `stateField`.
7. Deployment: the generated deployment diagram of each deployment whose
   servers are named; the environments, release and commissioning checks
   are in their stages.
8. Cross-cutting concepts: permissions table (generated), money, time,
   identifiers, error handling.
9. Architecture decisions: generated list from `decisions`, with a link to
   each.
10. Quality requirements: scenarios.
11. Risks and technical debt.
12. Glossary: the `glossary` section, explained where it needs it.
13. Requirements: the requirements and the traceability matrix are in the
    requirements stage and in the techspec; this section says where.

A section with nothing to say is left out; the others keep their numbers,
so section 8 is always cross-cutting concepts. A placeholder line would only
be noise for the reader.

## Mermaid diagrams: generated or hand-drawn

Generated diagrams sit between marker comments. A target replaces
everything between the markers and leaves the rest of the file alone. Never
edit inside the markers; edit the YAML.

    <!-- specarch:generate erDiagram -->
    ```mermaid
    erDiagram
      ...
    ```
    <!-- specarch:end -->

| Diagram | Marker | Source | Who draws it |
|---|---|---|---|
| `erDiagram` of entities, fields, keys, relations | `erDiagram` | `entities`, `relations` | generated |
| `stateDiagram-v2` of one entity with a `stateField` | `stateDiagram <Entity>` | `transitions` | generated |
| `sequenceDiagram` of one operation: client, service, algorithm, channels, answer | `sequenceDiagram <operationId>` | `paths`, `emits`, `algorithm` | generated |
| `sequenceDiagram` of one command: user, program, files read and written, exit statuses | `sequenceDiagram <command>` | `commands` | generated |
| `flowchart` of pages and their `actions` | `flowchart pages` | `pages` | generated |
| `flowchart` of one implementation's components: a node per layout entry, an edge per `dependsOn`, the libraries apart | `components <stack>` | an implementation file's `layout`, `dependsOn`, `libraries` (ADR-133) | generated |
| `flowchart` of one function of an operation's handler, a node per step by its id | | an implementation file's `mappings`, `handler` (ADR-144) | generated, in the technical specification only |
| permissions matrix (a Markdown table, not a diagram) | `permissions` | `roles`, `permissions` | generated |
| `flowchart` of the system's context in C4's style: a person per role, a system per service client, the dependencies by kind with their protocol, the channels they carry | `context` | `roles`, `clients`, `dependencies`, `channels`, `calls`, `emits`, `consumes` (ADR-134, ADR-135) | generated |
| `flowchart` of one deployment: a node per server with its name, kind, host and what it runs, a server that is a dependency drawn as that dependency | `deployment <deployment>` | an implementation file's `deployments`, `servers` with `name`, `kind`, `runs`, `dependency` (ADR-136) | generated |
| `flowchart` of one deployment's network: a subgraph per zone, an edge per link with its port and protocol, the dependencies no server is apart | `network <deployment>` | an implementation file's `deployments`, `zone`, `listens`, `links` (ADR-137) | generated |
| any explanatory sketch of an algorithm or a flow | | prose | hand-drawn |

`specarch document techspec` fills every marked region of the `specarch.md`
beside a root file, and writes the whole technical specification, every
generated diagram included, to `techspec.md` in the folder it owns. A marker
for something the specification does not have is an error. The example in
`examples/library-lending/` shows both.

## The rule for new keywords

Before adding a keyword to the meta-model, look for one in JSON Schema,
OpenAPI, AsyncAPI or the standard of that stage that means the same thing
and take it with its exact meaning. Add a SpecArch keyword only when none
exists, mark it in the schema description and add a row to the origin table
above.
