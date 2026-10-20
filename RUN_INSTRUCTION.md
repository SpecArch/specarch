# Run instruction: a specification from existing documents and code

How to turn a project folder that holds documents and unfinished code into
a validated SpecArch specification with the `specarch` command alone. No
AI is needed, and nothing is sent anywhere. The network is used only to
fetch: `go install` and `git clone` at the start, the PostgreSQL image
the database dump runs, and the packages a Swift or Dart reader fetches
on its first run. The method behind it is in
[`docs/extraction.md`](docs/extraction.md), and the procedure for a person
who writes the specification from both sources in
[`docs/from-sources.md`](docs/from-sources.md). This file holds the
commands.

## Can it be done?

Yes, for everything the sources state. The command reads each source with
a reader of its own, merges what the readers wrote, checks the result and
writes documents from it:

- `specarch extract <source>` reads one kind of source into a partial
  specification tree. Every element it writes cites the file and line, or
  the document section, it came from, or says why it was inferred.
- `specarch facts <language>` writes the code-facts dump that the
  JavaScript, TypeScript, Swift and Dart readers read.
- `specarch merge` joins the trees into one specification. Where a
  document and the code disagree, it writes a `must` question that cites
  both and leaves the disputed value out. It never picks one side.
- `specarch validate` checks the specification, and `specarch gaps` lists
  the open questions, the elements by origin, what each source section
  produced and which outputs can be made now.
- `specarch document <target>` writes what a person reads: the problems,
  the questions, the requirements, the technical specification and an
  HTML site.
- `specarch decide` records a person's answer to a question as a decision
  record, removes the question and prints the entries to write from the
  answer.

What it cannot do without a person is supply meaning the sources do not
hold: which of two disagreeing sources is right, what an undocumented
endpoint is for, which kind a requirement is, what a JSON column holds.
SpecArch is a compiler for specifications (`docs/principles.md`), so it
writes each of these as a question in the problems file, marked at its
entry, and never guesses a value. A person answers them with
`specarch decide` and writes the entries that `decide` names. Code
generation waits until no `must` or `should` question is left and the
owner has approved (`specarch approve`).

## What each reader reads

Every reader reads files in a git repository whose changes are all
committed: a change not committed, an untracked file under the path read,
a shallow clone or a path outside a repository is refused, because the
specification cites a commit. A dump is committed before it is read. The
readers read files and dumps; they never run the project.

| Source | Give it | It needs |
|---|---|---|
| `documents` | one Markdown file | Markdown only; convert a Word or PDF document first. English is read by default; for another language give `--vocabulary <file>` ([`docs/vocabularies.md`](docs/vocabularies.md)) and `--language <tag>`. `--given-outside` marks a document given to parties outside, such as a published manual |
| `openapi` | one OpenAPI 3.0 or 3.1 file, YAML or JSON | nothing more |
| `go` | Go source folders of one module | `--implementation <file>` naming the project's permission check, so that each route gets its permission; without it, each route's permission is a question. `--implementation-out <file>` writes each handler as a mapping |
| `javascript` | a dump from `specarch facts javascript <folder> <dump>` | nothing more; the TypeScript compiler is built into `specarch` |
| `swift` | a dump from `specarch facts swift <folder> <dump>` | a Swift toolchain; the first run builds the reader and fetches SwiftSyntax |
| `dart` | a dump from `specarch facts dart <folder> <dump>` | the Dart SDK; the first run fetches the analyzer package |
| `pages` | the root folder of a Next.js or Nuxt router (`app`, `pages` or `server`) | `--facts <dump>`, a JavaScript dump of a folder that holds the router's folder, to read the files' code; without it, the files' places and schema files only |
| `workflows` | one BPMN 2.0 XML file | for definitions in a format of the project's own, a workflow table from `tools/workflows/dump-workflows.sh` and `--table` |
| `database` | a PostgreSQL catalogue dump | `tools/catalogue/dump-catalogue.sh`, which applies the migrations to a disposable PostgreSQL under Podman. `--implementation-out <file>` writes each table's mapping and plain indexes |
| `router` | a route table | `tools/routes/dump-routes.sh` around a route printer the project writes, a small program that builds the router as the server does and prints its routes as JSON |
| `permissions` | a permission table | `tools/permissions/dump-permissions.sh` around the project's permission printer; `--routes <route table>` also writes each route with its permission |
| `gateway` | a gateway table | `tools/gateways/dump-gateway.sh` around the project's printer |
| `outline` | any files or folders | nothing; it lists them as clauses, so `gaps` shows them as not read yet |

The dump scripts are in this repository, not in the installed binary, so
clone the release beside the project. Each script's first lines give its
format and its printer's contract. Every code reader takes `--source-key
<key>` to name its source in the tree (default `code`; the file's name for
`documents` and `openapi`).

## The commands

These commands run in zsh and in bash, from the project's root. They
assume Go 1.27.2 or later, git, and Podman for the database and
permission dumps. Give your own paths in place of `docs/`, `code/`,
`web/`, `workflows/` and the others; any layout works, since every path
is an argument. Leave out the lines of a source your project does not
have. In sections 2 and 3 the first blocks are the sources of the worked
example below; the blocks headed "Other sources" cover the readers it has
no source for.

### 1. Install

    go install github.com/SpecArch/specarch/cmd/specarch@v0.7.2
    go version -m "$(go env GOPATH)/bin/specarch"
    SA="$HOME/specarch-v0.7.2"
    git clone --depth 1 --branch v0.7.2 https://github.com/SpecArch/specarch.git "$SA"

`go version -m` names the module version and its `h1:` checksum; v0.7.2
is `h1:jc5YweQS4R/jeioXTVI4y5Iy0hVP+XDWo28KzmOUEyM=`. The generator
plug-ins (`specarch-gen-sql` and the others, listed in the README) are only
needed later, to generate code from an approved specification.

### 2. Commit the sources, then make the dumps

The readers refuse uncommitted files, so commit first. The folder the
readers write, `trees/`, is ignored.

    printf 'trees/\n' >> .gitignore
    git add -A && git commit -m "Documents and code to specify"
    mkdir -p dumps
    specarch facts javascript web dumps/web.json
    sh "$SA/tools/catalogue/dump-catalogue.sh" code/migrations dumps/catalogue.json
    sh "$SA/tools/routes/dump-routes.sh" code dumps/routes.json go run ./cmd/routetable
    sh "$SA/tools/permissions/dump-permissions.sh" code dumps/permissions.json sh cmd/permissiontable/print.sh
    git add dumps && git commit -m "Dumps of the code"

`go run ./cmd/routetable` and `sh cmd/permissiontable/print.sh` are the
project's own printers; each runs in the folder given before the dump
file.

Other sources: a SwiftUI or Swift server folder (`ios/`), a Flutter or
Dart folder (`mobile/`), workflow definitions in a format of the
project's own (`flows/`, read by a printer of the project's) and a
gateway's table of services (`gateway/`):

    specarch facts swift ios dumps/ios.json
    specarch facts dart mobile dumps/mobile.json
    sh "$SA/tools/workflows/dump-workflows.sh" flows dumps/workflows.json sh ../tools/print-workflows.sh
    sh "$SA/tools/gateways/dump-gateway.sh" gateway dumps/gateway.json sh ../tools/print-gateway.sh
    git add dumps && git commit -m "Dumps of the other sources"

`sh ../tools/print-workflows.sh` and `sh ../tools/print-gateway.sh` stand
for the project's own printers, as above; each script's first lines give
what its printer writes.

### 3. Read each source

The go reader needs an implementation file that names the permission
check. Write it once (package, function and the position of the
permission argument, from your code):

    cat > checks.specarch-implementation.yaml <<'EOF'
    specarchImplementation: "0.1"
    info: { title: Permission checks, version: 0.1.0 }
    bindings:
      http:
        permissionChecks:
          - { package: example.com/lendingdesk/lending, function: Server.Require, permissionArgument: 1 }
    EOF
    git add checks.specarch-implementation.yaml && git commit -m "Name the permission check"

Then one reader per source, each into a tree of its own:

    specarch extract documents --out trees/manual docs/manual.md
    specarch extract openapi --out trees/openapi docs/openapi.yaml
    specarch extract database --implementation-out trees/database-mappings.yaml --out trees/database dumps/catalogue.json
    specarch extract router --out trees/router dumps/routes.json
    specarch extract go --implementation checks.specarch-implementation.yaml --out trees/go code
    specarch extract permissions --out trees/permissions dumps/permissions.json
    specarch extract pages --facts dumps/web.json --out trees/pages web/app
    specarch extract workflows --out trees/workflows workflows/write-off.bpmn

Other sources, each into a tree of its own as well:

    specarch extract javascript --out trees/web dumps/web.json
    specarch extract swift --out trees/ios dumps/ios.json
    specarch extract dart --out trees/mobile dumps/mobile.json
    specarch extract workflows --table --out trees/flows dumps/workflows.json
    specarch extract gateway --out trees/gateway dumps/gateway.json
    specarch extract outline --out trees/outline docs workflows

`extract javascript` reads Express and Fastify servers and React Router
and vue-router apps (`examples/room-booking` is one); for a Next.js or
Nuxt file router, `extract pages` above reads the same dump. The
`javascript`, `swift` and `dart` readers take `--implementation <file>`
for the permission check, as the go reader does. `extract outline`
lists every file it is given as a clause and reads nothing in it, so that
`gaps` shows what no reader has read yet.

The flags that change how a document is named or read go on its line, in
place of the plain `documents` line above:

    specarch extract documents --given-outside --source-key manual --language en --out trees/manual docs/manual.md
    specarch extract documents --vocabulary de.specarch-vocabulary.yaml --out trees/handbook docs/handbuch.md

`--given-outside` marks the manual as given to parties outside the
project. `--source-key manual` names the source `manual` in the tree, in
place of the file's name (for a code reader, in place of `code`).
`--language <tag>` reads with a vocabulary SpecArch ships (English only,
at v0.7.2); `--vocabulary <file>` reads with a vocabulary the project
writes, as [`docs/vocabularies.md`](docs/vocabularies.md) says, and the
same file goes to `merge --vocabulary`.

Each reader prints what it counted, what it wrote and every thing the
specification cannot hold, with the question that asks about it.

### 4. Merge, validate, read

The documents' trees go first, so the questions come in the order a person
reads the sources:

    specarch merge --out spec trees/manual trees/openapi trees/database trees/router trees/go trees/permissions trees/pages trees/workflows
    specarch validate spec
    specarch gaps spec
    specarch document problems --out review spec
    specarch document questions --out review spec
    specarch document requirements --out review spec
    specarch document techspec --out review spec
    specarch document html --out review spec

`validate` exits 0 when there are no errors. `gaps` exits 1 while a
`must` or `should` question is open, which is the normal state of a
specification read from sources. `review/` gets `problems.txt` and
`problems.sarif`, `questions.md`, `requirements.md`, `techspec.md` and
`index.html`. Add the tree of each other source to the merge, after the
documents' trees. A merge given `--vocabulary <file>` reads the
documents' days with that file's words, for documents not written in
English.

### 5. Answer the questions

An answer cites where it came from. Declare that once under `sources` in
`spec/specarch.yaml`:

      owner-answers:
        kind: interview
        title: Answers of the system owner
        date: "2026-10-21"

Then, for each question:

    specarch decide Q-39 --by system-owner --answer "21 days: the code is to change" --source owner-answers spec

`decide` writes `spec/design/decisions/ADR-001.yaml`, removes the question
and prints each entry the answer unblocks, such as
`#/requirements/MANUAL-2/statement: write statement from ADR-001; mark
#/requirements/MANUAL-2 with origin: decided and decidedIn: ADR-001`.
Write those in the file it names, then run `specarch validate spec`
again: it reports an error at each entry not written yet.

### 6. After the documents or the code change

Commit the change. Then, for each source that changed, make its dump
again (section 2) and read it into a new tree under `trees/`; reuse the
tree of every source that did not change. Merge into a new folder and
compare the two specifications. Here only the manual changed:

    git add -A && git commit -m "The change"
    specarch extract documents --out trees/manual-next docs/manual.md
    specarch merge --out spec-next trees/manual-next trees/openapi trees/database trees/router trees/go trees/permissions trees/pages trees/workflows
    specarch diff spec spec-next

`merge` writes a new specification from the trees and carries nothing
over from an earlier one. `diff` lists what changed, including the
decision records and decided entries that `spec` has and `spec-next`
lacks: copy those across, then validate again. `diff` also reports an
error while no release record exists for the version, which matters only
when a version is released.

## Worked example

`examples/lending-desk/sources/` in the clone holds a small public sample:
a desk manual, the OpenAPI file it was given, a Go service with
migrations, seeds and both printers, a Next.js front end and a BPMN
workflow. They disagree on purpose. To run the commands above on it,
make the project folder from the sample, then run sections 2 to 6 in it,
the "Other sources" blocks and the flagged `documents` lines left out:

    mkdir -p "$HOME/lending-desk/docs" "$HOME/lending-desk/workflows"
    cd "$HOME/lending-desk"
    S="$SA/examples/lending-desk/sources"
    cp "$S/manual.md" "$S/openapi/openapi.yaml" docs/
    cp -R "$S/code" code
    cp -R "$S/web" web
    cp "$S/workflows/write-off.bpmn" workflows/
    git init -q

In section 6, change the manual's loan period first, so there is a change
to read:

    sed -i.bak 's/loan period is 21 days/loan period is 14 days/' docs/manual.md && rm docs/manual.md.bak

What the run gives, at v0.7.2:

- The readers write 4 requirements and 1 entity from the manual; 6
  operations, 2 entities and 2 schemas from the OpenAPI file; 5 entities
  and their mappings from the catalogue; 7 operations and 4 permissions
  from the route table; 7 operations from the Go source, all 7 handlers
  followed through the named check; 2 roles from the permission table; 3
  pages from the router; 1 workflow and 1 role from the BPMN file.
- `merge` joins them. Where one tree answers another's question (the
  OpenAPI file gives what the route table cannot say, the route table
  gives each permission the OpenAPI file leaves open), it drops that
  question.
- `validate`: 0 errors, 24 warnings, 39 open questions. `gaps`: 26 must,
  10 should and 3 could; 25 elements stated and 32 inferred; 7 of the
  manual's 12 sections produced nothing.
- Among the must questions: Q-39, "Is it 21 days, as MANUAL-2 says, or 14
  days, as the check loans_loan_period of Loans has it?", and Q-38, on
  `GET /members/{cardNumber}`, which the code serves and no document
  mentions.
- `decide Q-39` writes ADR-001; once MANUAL-2's statement is written with
  `origin: decided` and `decidedIn: ADR-001`, `validate` reports 0 errors
  and 38 open questions.
- After the manual's change, `diff spec spec-next` lists MANUAL-2 and the
  manual's source as changed, and ADR-001 and the `owner-answers` source
  as present only in `spec`.

- Of the other lines, those with a source in the sample were run in a
  second copy of it: `extract javascript` counts the 6 files of `web`
  and writes nothing, since the front end is a Next.js router that
  `extract pages` reads; `extract outline` lists the 3 files of `docs`
  and `workflows`; `extract documents --given-outside --source-key manual
  --language en` writes the same 4 requirements and 1 entity as the plain
  line.

The sample has no Swift, Dart, workflow-table or gateway source. Their
lines in the "Other sources" blocks were run at v0.7.2 on the sources of
the conformance cases `spec/tests/extract-swift-screens`,
`extract-dart-models`, `extract-workflows-table` and `extract-gateway`,
copied into a new git project, with printers that print the cases'
tables: `extract swift` wrote 9 pages, 1 entity, 1 schema and 25
questions; `extract dart` 3 schemas, 1 enum and 7 questions;
`extract workflows --table` 1 workflow, 1 role and 7 questions;
`extract gateway` 1 gateway, 3 dependencies, 3 settings and 10
questions. The `--vocabulary` line was not run: SpecArch ships no
vocabulary but English at v0.7.2, and the file is the project's own.

`examples/lending-desk/extract.sh` runs the same readers and merge on the
committed sample in place.
