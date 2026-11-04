# SpecArch by situation

One place that says which feature to use in which real situation, for
people and for agents. Each situation says when it happens, what to run,
shows a short worked example with the output the program prints, names
what to watch for, and points at the deeper document. `specarch help
situations` prints the same situations as a short index, built into the
program; `specarch help <topic>` and `specarch help <command>` print each
area and each command in full. The same guide in Indonesian is
`docs/id/guide-by-situation.md`, with the same situation ids.

The examples run on the specifications in this repository: the library
lending example (`examples/library-lending`, complete, with records beside
it), the lending desk (`examples/lending-desk`, read from a manual and code
that disagree, with open questions) and the release watcher
(`examples/release-watcher`, two daemons under systemd), and on the
conformance cases under `spec/tests/`. A command's output is shown as the
program prints it; a long listing is cut, with a line saying what follows,
and an absolute path the program prints is shortened to the path from the
working folder.
The program speaks English in every language of this guide.

## Two realities every situation allows for

Requirements are never fixed. People leave things out, say things that were
not what they meant, and change their minds once they see the system. So a
specification here is never refused for being incomplete: it is read,
every gap is a problem or an open question at its entry, documents and
drafts are written from it anyway, and a change is a record with its cause.
Users attend late. The people who will use the system come to the user
acceptance test, not to the requirements meetings, and ask there for
changes. So every generated result carries a trace back to the element it
came from, a remark at the acceptance test is a change request that names
that element, and the validator warns of a role nobody has heard before
the acceptance test, so that the absent user is at least asked.

| Situation | Id |
|---|---|
| Starting a new system | `new-system` |
| Interviewing people | `interviewing` |
| Capturing from customer documents | `customer-documents` |
| Capturing from existing code and databases | `existing-code` |
| Tabular registers | `registers` |
| Daemons and service units | `daemons` |
| Recording a conversation and who was present | `conversation-record` |
| A requirement made more detailed | `requirement-detailed` |
| Generating a draft before approval to show users | `draft-before-approval` |
| Approving the specification | `approving` |
| Building the system from the specification | `building` |
| Keeping the specification and the code equal in CI | `ci` |
| A small change | `small-change` |
| A large change late in the life cycle | `large-change-late` |
| A user who was absent objects at the acceptance test | `absent-user-objects` |
| Approving again, with diff | `approve-again` |
| Release and release notes | `release` |
| A release of SpecArch moves the formats | `formats-move` |
| Agents driving SpecArch | `agents` |

<!-- situation: new-system -->
## Starting a new system

**When it happens.** Nothing exists yet: no code, no database, no old
document. Someone has an idea and a few people who know what they need.

**What to run.**

    specarch init --profile lean notes
    specarch validate notes
    specarch gaps notes
    specarch document techspec --out notes/docs notes

`specarch init` writes the smallest specification that validates, and
nothing else: every key beyond it is a decision nobody has made yet. The
lean profile makes a missing test a warning, so a small tool or an internal
daemon can be validated, documented and shown before its tests are written;
on the full profile, the default, every test the design implies is an
error until it is written. `specarch help start` shows a first entity and
operation and what lean relaxes; `specarch help examples` is a worked
example from the empty folder to a generated file; `specarch help format`
is the layout of the files; `specarch help conventions` the keywords.

**A worked example.** On a new folder:

    $ specarch init --profile lean notes
    notes/specarch.yaml
    specarch init: next, run specarch validate notes after each edit; specarch help start shows a first entity and operation that validate
    $ specarch validate notes
    notes/specarch.yaml:7:12: warning: /info/profile: profile: the specification is on the lean profile, so a test specarch derive would write and nobody has is a warning, not an error (test_golden_missing, test_case_missing), and a missing problem is asked as the first question of specarch gaps --next, not warned (problem_missing); every other rule is as on full. Write profile: full, or remove the key, to make them errors, and run specarch derive to write the tests [profile@specarch.yaml#/info/profile]
    specarch: 1 input checked: 0 errors, 1 warning

**What to watch for.** Write the stakeholders first, by role, since every
question names who decides and every record names who spoke; validate
warns `stakeholders_unconfirmed`, and every document carries a banner,
until a role confirms the list is complete with `stakeholdersComplete`
(by, date and, when there is one, source), so a role nobody listed is seen before the acceptance
test rather than at it. Five things a
first draft leaves out are errors on every profile: the entity's primary
key, the operation's summary, each response's description, the one
permission an operation needs (access is closed unless granted) and a
bounded integer. An unknown is an open question under `questions`, never
a placeholder in a value.

**Deeper.** `docs/principles.md` (what SpecArch is for), `docs/stages.md`
(the seven stages), `docs/conventions.md` (the tree and the keywords).

<!-- situation: interviewing -->
## Interviewing people

**When it happens.** The specification has open questions, and a person
who can answer them has given you an hour. Or an agent is in a chat with
the analyst and the analyst is in a chat with the customer.

**What to run.**

    specarch gaps --next examples/lending-desk
    specarch decide Q-3 --by desk-manager --answer "<the option chosen, in full>" --source <interview> examples/lending-desk/spec
    specarch validate examples/lending-desk

`specarch gaps` prints every open question by stage, who decides each,
what it blocks and which outputs are ready, drafts or waiting; `specarch
document questions` writes the same as a document. `gaps --next` picks the
one question to ask next: a must before a should, then the one holding up
more outputs. The procedure, one question at a time with two to five
options and one recommended, is `specarch help asking`; `specarch help
gaps` explains questions, decisions and approval.

**A worked example.** The lending desk has eight open questions:

    $ specarch gaps --next examples/lending-desk
    Next question in Lending Desk (examples/lending-desk/spec/specarch.yaml), ranked first of 8 open questions: Q-3 (must, decision)

    How is each requirement shown to be met?

    - Decided by: desk-manager
    - Blocks: #/requirements/LEND-1/acceptance, #/requirements/LEND-2/acceptance, #/requirements/LEND-3/acceptance, #/requirements/LEND-4/acceptance, #/requirements/LEND-5/acceptance, #/requirements/LEND-6/acceptance, #/requirements/LEND-7/acceptance
    - Holds up: techspec document, requirements document, testplan document, traceability document and html document
    - Options: none written; offer two to five
    - Insight: Neither the manual nor the code gives acceptance criteria, and the code has no tests to read them from.
    - Record the answer: specarch decide Q-3 --by desk-manager --answer "<answer>" examples/lending-desk/spec
    - Source: the root file declares no source of kind interview; declare an interview under sources for the answer to cite, and add --source with its key

    Put it to desk-manager alone, with one answer recommended and why; specarch help asking.

The desk manager chooses; the decision is recorded with the full text of
the option, never the number the person said. A specification that tracks
origin refuses a decision with no source:

    $ specarch decide Q-3 examples/lending-desk/spec --dry-run --by desk-manager --answer "Each requirement is shown met by one test of the operation it names."
    specarch decide: examples/lending-desk/spec tracks origin, so the decision must cite where the answer came from; record the answer with specarch log and give its ID with --record, or declare it under sources in specarch.yaml, as an interview with its date, and give it with --source

**What to watch for.** Never answer a question yourself, and never pick
between two sources that disagree: the person does. Run `gaps --next`
again after each answer; one answer settles or opens others. When the
person is not the role the question names, pass it on. An answer given in
a chat or a call is a communication record first (`conversation-record`),
and `decide --record` cites it.

**Deeper.** `docs/refinement.md` (open questions, origin, the decision that
closes a question), `docs/questions.md` (SpecArch's own open questions,
generated).

<!-- situation: customer-documents -->
## Capturing from customer documents

**When it happens.** The customer hands over what they have: a manual in
Word, rules in a PDF, a deck of slides, a spreadsheet of fields, an email
thread with the decisions in it. There is no specification, and the
documents disagree with each other in places.

**What to run.**

    specarch extract documents --out trees/manual sources/manual.md
    specarch extract documents --out trees/rules sources/rules.pdf
    specarch merge --out spec trees/manual trees/rules
    specarch validate spec
    specarch gaps spec
    specarch document comparison --out docs spec

`extract documents` reads one Markdown, PDF, Word (.docx), PowerPoint
(.pptx), Excel (.xlsx) or email (.eml) file into a partial specification,
each element citing its clause: a section number, a page, a slide, a cell
range or an attachment. `--vocabulary` names the words a document uses
for its kinds of sentence (a must, a shall) and `--language` its language.
`extract outline` records the files of a surface no reader reads yet,
each as a clause and no element, so `gaps` lists them as producing
nothing rather than losing them. `specarch merge` joins the trees, keeps both sides of a
disagreement as a question, and `document comparison` shows the old
documents against the refined specification: what was kept from each
clause, what was concluded, what a decision added, and the clauses nothing
cites.

**A worked example.** The lending desk's manual:

    $ specarch extract documents --out trees/manual examples/lending-desk/sources/manual.md
    commit 85f752f8ddf25f3f53c9196d923c07ac54521429: the last change to examples/lending-desk/sources/manual.md
    counted 7 headings, 8 numbered paragraphs, 17 sentences (4 commitments) and 1 table (1 table of fields): every ATX heading, every paragraph that starts with a section number such as 3.1, every sentence of a paragraph or list item, and every table, outside fenced code
    wrote 4 requirements, 1 entity and 2 questions: one per commitment that names its subject, one per table of fields, one per thing the document does not say, and one per thing the meta-model cannot hold
    derived 0 draft tests under tests, as specarch derive writes them: the success of each subject the design gives one, and each chosen case no test covers

Every element written carries `origin: stated` with the clause it came
from; what the manual does not say is a question, never a guess.

**What to watch for.** The source must be committed: a reader records the
commit it read, and refuses a file with changes not committed. An
encrypted PDF, a file that is not its format by extension, or a document in
a language no vocabulary covers is exit status 1 with the reason; `specarch
help extract` lists them. Nothing is invented to fill a gap: the questions
are the work left, and `interviewing` is how they are closed.

**Deeper.** `docs/extraction.md` (the surfaces and the readers),
`docs/from-sources.md` (the procedure an agent follows, documents first),
`docs/vocabularies.md`.

<!-- situation: existing-code -->
## Capturing from existing code and databases

**When it happens.** The system runs, and its specification is the code.
A rewrite, a handover or an audit needs the as-built specification, and
the documents that exist say something else.

**What to run.**

    specarch extract database --out trees/database sources/catalogue.json
    specarch extract router --out trees/router sources/routes.json
    specarch extract openapi --out trees/openapi sources/openapi.yaml
    specarch extract permissions --out trees/permissions sources/permissions.json
    specarch extract go --out trees/go sources/code
    specarch facts javascript sources/web sources/facts/web.json
    specarch extract pages --out trees/pages sources/web/app --facts sources/facts/web.json
    specarch extract workflows --out trees/workflows sources/workflows/write-off.bpmn
    specarch extract gateway --out trees/gateway sources/gateway.yaml
    specarch merge --out spec trees/*

One reader per surface: `database` reads a catalogue dump, `router` a
route table the built router printed, `openapi` an OpenAPI document,
`permissions` the table the running check reads, `pages` a file-system
router's folder, `workflows` BPMN 2.0, `gateway` a gateway's table, and
`go` parses Go source on dxlib, `net/http` and the common routers. Swift,
JavaScript and Dart are read through a code-facts dump that `specarch
facts <language>` writes from a committed folder, read by `extract swift`,
`extract javascript` and `extract dart`. `examples/lending-desk/extract.sh`
runs every reader on the example and merges them.

**A worked example.** The merge of the lending desk's eight trees:

    $ sh examples/lending-desk/extract.sh out
    ...
    extract.sh: wrote out/spec; specarch: 1 input checked: 0 errors, 24 warnings, 39 open questions
    $ head -4 out/merge.txt
    tree trees/manual: Lending desk manual, on the documents side: 6 elements and 2 questions
    tree trees/openapi: Lending desk API, on the documents side: 11 elements and 10 questions
    tree trees/database: Database of examples/lending-desk/sources/code/migrations, on the code, printed side: 6 elements and 4 questions
    tree trees/router: Routes of examples/lending-desk/sources/code, on the code, printed side: 12 elements and 15 questions

Then merge.txt says, one line each, which question one tree asked that
another answers (`answered: Q-2 of trees/manual, on
#/entities/Members/primaryKey, is left out; Database of
examples/lending-desk/sources/code/migrations gives it`), which two trees
both ask (`asked twice`), and which the merge joined.

**What to watch for.** A source read at a commit is pinned to it: a dump
remade from another commit, a shallow clone or a path outside a repository
is refused with the reason. Where the manual and the code disagree, both
stay, as a question for the owner; the agent never picks. The implementation
file, what is stack-specific, is written from the code by hand
(`docs/from-sources.md`, 3.4). After the specification is approved,
`specarch check` keeps it equal to the running code (`ci`).

**Deeper.** `docs/extraction.md`, `docs/reading-code.md` (what each
language reader takes and what stays with a printer),
`docs/from-sources.md`, `docs/gateway.md`.

<!-- situation: registers -->
## Tabular registers

**When it happens.** Part of the system is a spreadsheet people fill in: a
register of loans per quarter, a list of assets per site, one file per
period, with a template and an instructions sheet somewhere. Its columns
are the entity nobody wrote down.

**What to run.**

    specarch extract tables --out spec docs/loans-2026-q1.csv docs/loans-2026-q2.csv
    specarch extract tables --out spec --entity Loan --template docs/loans-template.xlsx --instructions docs/loans-instructions.xlsx docs/loans-2026-q1.csv
    specarch generate register --out registers spec
    specarch check spec docs/loans-2026-q3.csv

`extract tables` reads CSV, TSV and Excel files of one register into one
entity whose fields are the columns, each type, whether it is required and
its values inferred from every value of every file and cited at the file,
sheet and column; the template and the instructions give the fields'
statement. `generate register`, the plug-in `specarch-gen-register`, writes
the register back as the template people fill in, and `specarch check`
compares a filled file with its entity.

**A worked example.** One quarter's file, written with a byte order mark,
CRLF line ends and semicolons:

    $ specarch extract tables --out spec docs/loans-2026-q1.csv
    commit 2ccb823f3b898bfa12e0b7870ea3df72a8f08169: the last change to loans-2026-q1.csv
    dialect loans-2026-q1.csv: UTF-8 with a byte order mark, CRLF line ends, delimiter ';', no quoted field
    counted 1 file, 1 table, 12 rows and 6 columns: every file given, every register sheet or file as a table, its first row the header and every other row a record, and every header that names a field as a column, once per register
    entity Loans2026Q1: 6 fields from 12 rows in loans-2026-q1.csv; candidate keys loanId, dueDate
    wrote 1 entity and 2 questions: one per entity's primary key, one per column whose values do not settle its type, whether it is required or its values, or that some files or the template lack, one per header that names no field, and one per thing the files hold that is not read
    derived 0 draft tests under tests, as specarch derive writes them: the success of each subject the design gives one, and each chosen case no test covers

**What to watch for.** The primary key is a must question naming the
candidate columns, since values alone cannot say which column is the key.
A column whose values read as two kinds stays a string with a question at
its type, citing the rarer cells. Give the files of one register together,
or `--entity` names them; a template given to a run of several registers
is refused.

**Deeper.** `docs/extraction.md` (registers), `docs/generators.md` (the
register target), `specarch help extract`.

<!-- situation: daemons -->
## Daemons and service units

**When it happens.** The system has no users at a screen: it is a process a
service manager starts and watches, a job a timer runs each night, a
watcher that polls a registry. Nobody calls it, so nothing in it needs a
permission, and what it needs is a restart rule, its signals, its exit
statuses, its settings and a monitor.

**What to run.**

    specarch extract systemd --out spec sources/units/release-watcher.service sources/units/release-prune.service sources/units/release-prune.timer
    specarch validate spec
    specarch document operations --out docs spec
    specarch document deployment --out docs spec

A process is written under `processes`: how it runs (always, scheduled or
on demand), the jobs it runs, the settings it reads, the signals and exit
statuses it honours, when it is restarted and the limit after which it is
given up, its health and its monitors. `extract systemd` reads `.service`
and `.timer` units into them, citing each line. The operations guide
writes the daily checklist, each monitor with the response when its
objective is missed, and the backup, restore and rollback steps; the
deployment guide the environments and the hosts.

**A worked example.** The release watcher's three units:

    $ specarch extract systemd --out spec examples/release-watcher/sources/units/release-watcher.service examples/release-watcher/sources/units/release-prune.service examples/release-watcher/sources/units/release-prune.timer
    commit f4ad62a18e50e825f89506a210ee6c5e4745d3bf: the last change to examples/release-watcher/sources/units/release-prune.service, examples/release-watcher/sources/units/release-prune.timer, examples/release-watcher/sources/units/release-watcher.service
    counted 2 service units and 1 timer unit in examples/release-watcher/sources/units/release-prune.service, examples/release-watcher/sources/units/release-prune.timer, examples/release-watcher/sources/units/release-watcher.service
    wrote 2 processes, 2 settings and 14 questions: one process per service unit, its time from its timer, one setting per variable Environment= sets, and one question per thing the units leave to the host or do not say
    not held: examples/release-watcher/sources/units/release-prune.service:6: [Service] ExecStart= says how this host runs the process, which an implementation file's deployment holds, not the design; could question Q-6
    ... 8 more lines of what the units say that the design does not hold
    not derived: 6 draft tests for process releaseWatcher, held up by Q-2, Q-3; the validator warns for them until the questions are answered

**What to watch for.** What a unit leaves to the host's defaults is a
question: the restart limit when none is written, what each exit status
means, the schedule a calendar expression gives on a host whose clock is
UTC. `ExecStart=`, `User=` and `WantedBy=` are how one host runs it,
which belongs in the implementation file's deployment, not the design. A
secret in an environment file becomes a setting, never its value. The lean
profile suits an internal daemon until its tests are written.

**Deeper.** `examples/release-watcher/README.md`, `docs/conventions.md`
(Processes), `docs/deployment.md`.

<!-- situation: conversation-record -->
## Recording a conversation and who was present

**When it happens.** The analyst talked with a stakeholder, by meeting,
chat, email or call, and reports it in the chat in free words: "I asked
about the limit; the board chair answered five at 10:15 by chat". Later
somebody will ask what was said, by whom, when, and whether the people who
use the system were ever in the room.

**What to run.**

    specarch log spec --recorded-by analyst --direction in --from library-board --to analyst --person "the board chair" --at 2026-10-31T10:15:00+07:00 --channel chat --type answer --words "Five at most." --form verbatim --question Q-1
    specarch decide Q-1 --by library-board --answer "Five at most." --record COM-1 spec
    specarch validate spec
    specarch document communications --out docs spec
    specarch document timeline --out docs spec

`specarch log` writes one communication record, `records/communications/
COM-<n>.yaml`, from its options alone: who (roles, and a person label the
documents for everyone leave out), when (an instant with its offset), how
(channel), what (answer, question, request, information, objection), the
words and whether they are verbatim or a summary, the evidence file under
`records/` with its SHA-256, and what it touches. A meeting is a session
record with who was invited and present; a test session and a deploy are
records of their own. `document communications` is the history per
stakeholder and per requirement; `document timeline` every event, from the
records and the specification, in one time order.

**A worked example.** Half the facts given:

    $ specarch log spec --recorded-by analyst --from member --to analyst --type answer --words "five at most" --form summary
    missing --direction: ask which way it went: did the stakeholder say it to the project (in), or the project to the stakeholder (out)?; one of in, out
    missing --at: ask when it was said: the date, the time and the offset of the clock it was read on (the specification's time zone is UTC); written as 2026-10-31T10:15:00+07:00
    missing --channel: ask how it was said; one of meeting, chat, email, call, document
    missing --question: ask which question of the specification it answers, by ID
    specarch log: 4 options not given; ask the analyst what each line above asks, and run it again with every option; nothing written

All of them given, the record is written and the decision line printed:

    $ specarch log --direction in --from library-board --to owner --person "the board chair" --at 2026-10-31T10:15:00+07:00 --channel chat --type answer --words "Must: a release without the limit is not acceptable. It is shown met by refusing a sixth loan." --form verbatim --recorded-by owner --evidence records/evidence/COM-1-chat.txt --question Q-1 --element CASE-1 project
    records/communications/COM-1.yaml
    next: specarch decide Q-1 --by library-board --answer "Must: a release without the limit is not acceptable. It is shown met by refusing a sixth loan." --record COM-1 project

**What to watch for.** The command guesses nothing: a time without a date
or an offset is asked back, a role is one of the specification's
stakeholders and never a name, and the agent asks the analyst exactly what
each `missing` line says. An evidence file changed after it was recorded
fails validate with the hash it has now. A call with no minutes is still a
record, marked as recorded without evidence. From the first day,
validate warns `unheard` at a stakeholder an accepted requirement needs
that no record shows was heard (`absent-user-objects`); while no record is
kept yet, the warning says to start by logging the first conversation.

**Deeper.** `docs/maintenance.md` (Communications, sessions, test sessions
and deploys), `specarch help asking` (Recording what the analyst reports
from a conversation), `specarch help log`.

<!-- situation: requirement-detailed -->
## A requirement made more detailed

**When it happens.** A requirement was one sentence. Now the entity it
needs, the operation that carries it out, the rule it implies and the test
that shows it is met have to be written, and the sentence has to stay
traceable to each.

**What to run.**

    specarch validate spec
    specarch derive --dry-run spec
    specarch derive --fill spec
    specarch try lateFee spec
    specarch document traceability --out docs spec
    specarch document testplan --out docs spec

An entity, an operation, an algorithm or a page says `satisfies: [LIB-5]`;
a test says `verifies: [LIB-5]`; the validator warns
`requirement_unverified` at a requirement nothing verifies. `specarch
derive` writes a draft test for every case the design implies that no test
covers (a field missing, a value of the wrong form, a caller without the
permission) and `--fill` adds the fixture, input and expect the design
fixes. An algorithm carries worked examples, and `specarch try` evaluates
its formula on each and prints every step. The traceability document is
the matrix from needs through requirements to elements, tests and checks;
the test plan the cases, golden and red, traced to requirements.

**A worked example.** A requirement with no test yet, and an algorithm with
four examples:

    $ specarch validate examples/lending-desk
    examples/lending-desk/spec/requirements/requirements.yaml:53:3: warning: /requirements/LEND-4: requirement_unverified: no test, commissioning check or monitor verifies requirement LEND-4; add verifies: [LEND-4] to the test that shows it is met [requirement_unverified@requirements/requirements.yaml#/requirements/LEND-4]
    specarch: 1 input checked: 0 errors, 1 warning, 8 open questions
    $ specarch try lateFee examples/library-lending
    lateFee: 4 examples, 4 pass
    example 1 "returned on time": pass
      inputs: daysLate 0, dailyRate "0.50", replacementCost "25.00"
      decimal(daysLate, 0) = "0"
      decimal(daysLate, 0) * dailyRate = "0.00"
      min(decimal(daysLate, 0) * dailyRate, replacementCost) = "0.00"
      result "0.00", expected "0.00"
    ... 3 more examples

**What to watch for.** Detail that comes from a decision carries `origin:
decided` and `decidedIn`; detail concluded from evidence carries `origin:
inferred` and `why`. A worked example is the first test of an algorithm:
take its numbers from a real record. On the full profile a subject without
its golden test is an error, so write the test with the element, or let
`derive` draft it. Types are concrete and width-aware: an `int64` sent as
a JSON number loses digits above 2^53 in JavaScript.

**Deeper.** `docs/test-generation.md`, `docs/traceability.md` (SpecArch's
own matrix, generated), `docs/conventions.md` (Types), `specarch help try`,
`specarch help validate`.

<!-- situation: draft-before-approval -->
## Generating a draft before approval to show users

**When it happens.** The specification is half written and the questions
are open, but the users understand a screen better than a document, and
the sooner they see one the sooner they say what is missing.

**What to run.**

    specarch document html --out docs spec
    specarch document techspec --out docs spec
    specarch generate ui --unapproved --out drafts/web spec
    specarch generate ui --unapproved --environment staging --out drafts/staging spec

Documents are written for an incomplete specification, each problem and
each open question marked at its element; `document html` is the whole
specification as one page that needs no server, with each generated
screen in it. `generate --unapproved` makes code from a specification not
yet approved, every file marked a draft. An environment whose entry says
`trace: true` gives a build that shows the trace: the web UI names each
page, section, field, column and action's element, draws an overlay with
the element under the focus, the build's version and digest and a remark
to copy as a change request; the Go server answers with the header
`Specarch-Element`.

**A worked example.** The library's staging environment shows the trace,
and a traced build goes into a folder of its own:

    $ specarch generate ui --dry-run --unapproved --environment staging examples/library-lending
    specarch generate: staging is an environment of examples/library-lending/spec whose entry says trace: true, and its build would go into examples/library-lending/web, the output folder the implementation file names; that folder holds the untraced build, the one generate --check compares, so a build that shows the trace goes under --out: give --out <folder> and run it again
    $ specarch generate sql --dry-run --unapproved examples/library-lending
    specarch generate: 0 files to write, 1 file already current; nothing written (--dry-run)

**What to watch for.** A draft is not for anything that will be used: the
files say so in their header. The trace names the inside of the system, so
only an environment that asks for it shows it, never production. Every
generated file names the specification's version and digest, so a remark
on the draft can say which build it was about. A remark comes back as a
change request (`absent-user-objects`), never as an edit to the generated
file, which the next generation overwrites.

**Deeper.** `docs/acceptance.md` (1, Show a result early; 2, Trace a
result to the specification), `specarch help generate`.

<!-- situation: approving -->
## Approving the specification

**When it happens.** The must and should questions are answered, the
documents have been written and the person who decides has read them. Code
for use may be generated only from a specification that person approved.

**What to run.**

    specarch document techspec --out docs spec
    specarch document requirements --out docs spec
    specarch document problems --out docs spec
    specarch approve --dry-run --by librarian --session SES-1 spec
    specarch approve --by librarian --session SES-1 spec

The approval record, `records/approvals/<version>.yaml`, holds the digest
of every file, so any later change voids it, and the commit it was made at
when every file is committed. `approve` refuses a specification with
errors, with an open must or should question, or whose documents differ
from what it says now, and a role never heard: no communication, session,
interview or decision names it (the rule `unheard`). `--session` names the
session, test session or communication record in which the role read the
documents, and it must list the role; without it `approve` warns and goes
on. `document problems` writes `problems.txt` and
`problems.sarif`, every error, warning and open question one line each,
for the approver to read first.

**A worked example.**

    $ specarch approve --dry-run --by librarian --session SES-1 library
    would write library/records/approvals/0.1.0.yaml
    specarch approve: version 0.1.0 of library/spec can be approved by librarian; nothing written (--dry-run)

**What to watch for.** Only the person's own word runs `approve`: an agent
asks and stops. Approval is of exactly what was read, whole: a part of a
specification cannot be approved, and the cost of approving again is kept
down by `diff` (`approve-again`). A folder with changes not committed gets
an approval without a commit, with a line on standard error saying why.

**Deeper.** `docs/refinement.md` (Approval, The gate on code generation),
`specarch help gaps`, `specarch help approve`.

<!-- situation: building -->
## Building the system from the specification

**When it happens.** The specification is approved, a stack is chosen, and
the migrations, the API, the server binding, the screens, the tests and
the development data are to be written from it rather than by hand.

**What to run.**

    specarch generate sql spec
    specarch generate seed spec
    specarch generate sqlc spec
    specarch generate scripts spec
    specarch generate openapi spec
    specarch generate go-dxlib spec
    specarch generate tests spec
    specarch generate ui spec
    specarch generate bpmn spec
    specarch generate deploy spec
    specarch idioms spec
    specarch idioms diff type-rendering spec
    specarch generate ui --estate estate/specarch-estate.yaml estate/reading-room

Each target is a plug-in on PATH, `specarch-gen-<target>` or
`specarch-gen-<target>-<stack>` for a stack of its own: `specarch-gen-sql`
(migrations for PostgreSQL, SQL Server, Oracle or MariaDB),
`specarch-gen-seed` (development rows), `specarch-gen-sqlc` (queries),
`specarch-gen-scripts` (guarded data scripts), `specarch-gen-openapi`,
`specarch-gen-go-dxlib` (the Go server binding), `specarch-gen-tests-go`,
`specarch-gen-tests-swift` and `specarch-gen-tests-dart` (the tests of
the worked examples and the design tests), `specarch-gen-ui` (plain
JavaScript), `specarch-gen-ui-typescript` (Next.js and Carbon),
`specarch-gen-ui-swiftui` (the iPhone), `specarch-gen-ui-vue-htm` (the web
with no build step), `specarch-gen-bpmn` (each workflow as BPMN and SVG),
`specarch-gen-register` (`registers`) and `specarch-gen-deploy` (each
deployment as a Compose file, Kubernetes manifests or a cloud-init file
per host, the release, rollback and migration runbooks, and the script of
a fresh environment that builds, loads, tests and tears down a Compose
deployment, every secret referenced and never written). The implementation file under
`implementation/<stack>/` names the stack, each target's output folder
and the idioms it overrides; `specarch idioms` lists them and `idioms
diff` prints a shipped idiom's parts beside the override's. `--estate`
brings the modules of an estate into the run, so references into them
resolve. `specarch help plugins` lists the plug-ins found on PATH and what
each writes and does not write.

**A worked example.** The library's Go implementation uses fifteen shipped
idioms:

    $ specarch idioms examples/library-lending
    examples/library-lending/spec/implementation/go/library-lending.go.specarch-implementation.yaml
      audit-fields 1.1.0: shipped
      authorization-check 1.0.0: shipped
      background-jobs 1.0.0: shipped
    ... 12 more idioms, then the JavaScript, Swift and TypeScript files

**What to watch for.** A plug-in writes what its target covers, marks each
entry it could not write at its place, and names what it never writes; the
rest is written by hand, in files the target does not write, since
`generate` writes every file the plug-in answers (sqlc and scripts never
write over a file they did not write). `sql` refuses a change it cannot
tell from a rewrite, with what to do by hand. Nothing stack-specific goes
into the design: it belongs in the implementation file.

**Deeper.** `docs/generators.md` (every target and the plug-in protocol),
`docs/idioms.md`, `docs/ui-design.md`, `docs/ui-nextjs-carbon.md`,
`docs/ui-swiftui.md`, `docs/ui-vue-htm.md`, `docs/estate.md`,
`docs/dxlib-lessons.md`.

<!-- situation: ci -->
## Keeping the specification and the code equal in CI

**When it happens.** Somebody edits a generated file by hand, or changes
the database without changing the specification, and nobody notices until
the next generation overwrites it or the next release breaks a client.

**What to run.**

    specarch validate spec
    specarch generate sql --check spec
    specarch document techspec --check --out docs spec
    specarch check --allowlist check-allowlist.txt --since HEAD spec routes.json permissions.json catalogue.json
    specarch skill --out .agents/skills --check

`generate --check` and `document --check` make the output in memory and
exit 1 when a file on disk differs. `specarch check` compares the
specification with the dumps of the running code (its routes, its
permissions, its database catalogue, a register file) and fails on a
difference the allowlist does not name. `skill --check` fails when the
agent skill in the repository is stale.

**A worked example.**

    $ specarch generate sql --check --unapproved examples/library-lending
    specarch generate --check: 1 file checked, 0 files differ
    $ specarch check --allowlist service/check-allowlist.txt --since HEAD project service/routes.json service/permissions.json service/catalogue.json
    specarch check: 3 dumps compared: 0 differences, 3 allowed by the allowlist, 2 reports, 0 allowlist lines that fail
    project/specarch.yaml:27:7: warning: /entities/Copy/constraints/copies_title_given: check_expression_differs: the check copies_title_given of copies reads (title <> '') in the specification's table and (((title)::text <> ''::text)) in the database; the database rewrites a check's text, so this is a report [check_expression_differs copies.copies_title_given]
    service/catalogue.json:203:17: warning: /tables/2/indexes/1: index_not_listed: the index copies_title_lower of copies, CREATE INDEX copies_title_lower ON public.copies USING btree (lower((title)::text)), is in the database and the mapping of entity Copy does not list it; an index is physical design, so this is a report: list it under the mapping's indexes when it is plain [index_not_listed copies.copies_title_lower]

**What to watch for.** A file without the generated header is read as
built by hand: `--check` says whether it matches the output but for the
header. A report is a difference that is not a failure (the database
rewrote a check's text; an index is physical design); a difference is.
Run the checks before each commit and in CI, and the Swift validator beside
the Go one when both are built: they print the same output.

**Deeper.** `docs/sync-gates.md`, `scripts/landing-check.sh` (every check
this repository runs), `specarch help check`.

<!-- situation: test-scripts -->
## Bringing in the test scripts a team already has

**When it happens.** The team, or the customer, already has test cases:
Gherkin feature files, or a test sheet with a precondition, steps, test
data and an expected result per row. Some rows have no expected result,
some scenarios stop before their Then, and none of it is in the
specification.

**What to run.** Commit the scripts, then read them into a tree and merge
it with the others:

    specarch extract tests --out trees/tests scripts/notes.feature scripts/regression.xlsx
    specarch merge --out spec trees/code trees/openapi trees/tests
    specarch gaps --next spec

`extract tests` writes one test per scenario, example or row, at level
system, red when the script says negative or its expected result names a
refusal or an error, `origin: inferred` and citing its line or its row.
Each test gets a must question for the subject it exercises, naming the
request the script names (`POST /notes`), and one must question for each
given, step or expected result the script leaves out. `merge` keeps these
tests whole with their questions. Answer each with `specarch decide`,
write what the answer settles into the test, and the test joins the test
package.

**A worked example.** A scenario with no Then and a sheet row with nothing
but its ID and title (`spec/tests/extract-tests-incomplete-scripts`):

    read notes.feature: 4 tests
    read sheet.csv: 2 tests
    wrote 6 tests and 10 questions: one per subject a test exercises, which a script cannot name in the specification's words, 1 for a test with no given, 1 with no step and 2 with no expected result

**What to watch for.** A test is never given a part its script does not
say: a scenario with no expected result stays golden until the question
about it is answered. The subject question blocks the whole test, so the
test is held back from the package until it is answered. Only English
Gherkin keywords are read; a feature file with a `# language:` header is
read as text with no steps.

**Deeper.** `docs/extraction.md` (Test scripts), `specarch help extract`,
ADR-235.

<!-- situation: test-package -->
## Running the tests in a customer's environment

**When it happens.** The customer runs the acceptance environment, has
neither the code nor the project's test harness, and asks which paths of
the specification work there: the golden paths, the red paths and the
ones no test covers yet.

**What to run.** Make the package for that environment, hand it over, and
let whoever runs the environment run it with their own credentials:

    specarch generate tests --package acceptance --out package spec
    specarch run --callers callers.yaml --setup ./put-records --results records/evidence/TR-4-junit.xml package
    specarch log test-run --results records/evidence/TR-4-junit.xml --environment acceptance --at 2026-11-04T10:00:00+07:00 --by customer-it spec

The package, `specarch-test-package.yaml`, holds every test and every
derived case no test covers, each as a request to the environment's
servers with what its answer must show, or with the reason it cannot run
from outside. `callers.yaml` maps each role to the headers that sign it
in; the setup program puts a case's records in place. Neither goes into
the package or the results. The JUnit XML maps back through `log
test-run`, a derived case to its subject's element.

**A worked example.** The package of a kept note
(`spec/tests/generate-tests-package`), and its run against a server that
does not answer (`spec/tests/run-package-no-answer`):

    specarch generate: held back keep-note-seen, which Q-1 holds up; it joins the package once answered
    error: list-notices-succeeds: GET http://127.0.0.1:1/notices got no answer: Get "http://127.0.0.1:1/notices": dial tcp 127.0.0.1:1: connect: connection refused

**What to watch for.** A case with a role the callers file does not give,
or records and no setup program, is skipped with that reason, not failed.
A test an open question holds up is left out until it is answered. The
state after and the messages emitted are not checked from outside; the
project's own suite checks them. With several servers in the
environment, name one with `--server` or give `--base-url`.

**Deeper.** `docs/test-generation.md` (The test package),
`specarch help run`, ADR-233.

<!-- situation: test-results -->
## Keeping the test results of each build

**When it happens.** CI runs the tests on every build and throws the
results away. At the acceptance test the customer asks which requirements
were shown met on the build in front of them, and which part of the system
keeps failing from one build to the next.

**What to run.** Put the runner's JUnit XML file under `records/`, then:

    specarch log test-run --results records/evidence/TR-1-junit.xml --environment staging --version 0.2.0 --build "library-lending 0.2.0 at 5d3e8b1" --at 2026-10-14T09:30:00Z --by library-it spec
    specarch document testruns spec

`log test-run` writes `records/test-runs/TR-<n>.yaml`: the run's
environment, version, build and instant, who ran it, the file as its
evidence with its SHA-256, and one result per testcase mapped to its test
of the specification, so to the element the test is about and the
requirements it verifies. `--open-defect <test>` with `--severity` and
`--phase` opens a defect for a failure you judge to be one.
`document testruns` lists the runs and the failures per element and per
requirement, one column per run; the `timeline` places each run.

**A worked example.** The library lending example's first run, in
`examples/library-lending`:

    $ specarch log test-run --results records/evidence/TR-1-junit.xml --environment staging --version 0.2.0 --build "library-lending 0.2.0 at 5d3e8b1" --at 2026-10-14T09:30:00Z --by library-it spec
    records/test-runs/TR-1.yaml
    specarch log test-run: TR-1 written to records/test-runs/TR-1.yaml, 5 results: 4 passed, 1 failed, 0 errored, 0 skipped, 0 unmapped

and in `docs/testruns.md`, after the second run:

    | Requirement | TR-1 | TR-2 | Open defects |
    |---|---|---|---|
    | LIB-3 | 0 | 0 | none |
    | LIB-5 | 1 | 0 | none |

A testcase the specification has no test for is kept, not dropped
(`spec/tests/log-test-run-writes-record`):

    unmapped: TestRenewal: no test of the specification is named so, nor Test or test and its name in Pascal case, nor does the name begin with a test's name and a colon; nor is it a test the generators write for an algorithm's or a call mapping's worked examples, nor a case no test covers as a test package names it

**What to watch for.** A testcase maps by the names the test generators
write (`TestReturnLate`, `testReturnLate()`, `return-late: ...`), and the
generated tests of an algorithm's or a call mapping's worked examples
(`TestAlgorithmLateFee`, `algorithm lateFee: ...`) name that element under
`element`; a runner that names tests otherwise leaves results unmapped,
and `validate` warns `test_unmapped` at each until `test` or `element` is
written into the result. Nothing
is read from the file's timestamp, which has no offset: give `--at`. A
failure opens no defect by itself, since a flaky host fails a test too.
Keep the results file once it is recorded: the `evidence` rule fails when
it changes.

**Deeper.** `docs/maintenance.md` (Test runs), `specarch help log test-run`,
ADR-230.

<!-- situation: comparing-implementations -->
## Choosing between two implementations

**When it happens.** A team builds two implementations of one design, two
stacks or two libraries for one stack, and must pick one. The evidence is
in CI logs, scan reports and what people remember of each build, and the
decision that picks one cites none of it.

**What to run.** Keep each build's inputs under `records/`: its test run
(`specarch log test-run`) or its JUnit XML file, the grype or osv-scanner
JSON of its software bill of materials, and a measurements file, JSON with
`buildSeconds`, `linesOfCode`, `deploySteps` and, per requirement,
`latency` and `throughput` as `{value, unit, how}`. Then:

    specarch compare --test-run lending.go=TR-3 --results lending.swift=records/evidence/swift-junit.xml --scan lending.go=records/evidence/go-grype.json --scan lending.swift=records/evidence/swift-osv.json --measurements lending.go=records/evidence/go-measurements.json --measurements lending.swift=records/evidence/swift-measurements.json --out records/comparisons/lending spec spec/implementation/go/lending.go.specarch-implementation.yaml spec/implementation/swift/lending.swift.specarch-implementation.yaml

Each option names an implementation by its file name without
`.specarch-implementation.yaml`. `compare` writes one table, one column
per implementation: the derived cases passed, the results, the scan
findings by severity, the libraries with their licences, the build time,
the lines of code and the deploy steps; then a row per derived case, per
measured requirement beside its statement, per library and per finding;
the inputs with their SHA-256; and the gaps. With `--out` it writes
`comparison.md` and `comparison.json` and prints the `sources` entry a
decision cites it by.

**A worked example.** `spec/tests/compare-two-implementations`:

    | Summary | lending.go | lending.swift |
    |---|---|---|
    | Derived cases passed | 0 of 2 | 1 of 2 |
    | Build time | 74 s | gap: not measured |

and on standard output:

    to cite the comparison from a decision, declare it under sources in specarch.yaml:
      comparison-lending: {kind: "document", title: "Comparison of lending.go and lending.swift", edition: "sha256:...", url: "../records/comparisons/lending/comparison.md"}

**What to watch for.** A missing result is a gap, never a zero: it is
written `gap` in its cell and listed with the option or key that fills
it, so read the Gaps section before the summary. Latency and throughput
stand beside the requirement's words, not judged against them. A test run
names no implementation, so you say which run is whose each time. An
input that is given but wrong is refused with exit 1, not shown as a
gap. The edition is the SHA-256 of `comparison.json`: run `compare`
again after an input changes, and update the entry.

**Deeper.** `specarch help compare`, ADR-232.

<!-- situation: small-change -->
## A small change

**When it happens.** A field gains a length, a list gains a column, a
message is reworded. Approved, in use, and the change is a morning's work.

**What to run.**

    specarch validate spec
    specarch diff spec-at-approval spec
    specarch generate sql --check spec
    specarch document changes --out docs spec

A change is a record first, `records/changes/CR-<n>.yaml`: who raised it,
when, its phase (development, acceptance or production), its type and
urgency, its `affects` by stage and its `impact` (patch, minor or major,
by what it does to the public interface). Then the specification changes,
`validate` checks it, `generate --check` shows which generated files are
now stale, and `diff` lists what changed between two versions of the
specification and checks the release between them.

**A worked example.** Three changes between two versions of a case:

    $ specarch diff old/spec new/spec
    minor changed #/entities/Item: properties/weight is added
    minor added #/paths/~1items~1{itemId}/get
    patch changed #/requirements/CASE-1
    specarch diff: 3 elements changed, 0 errors, 0 warnings

**What to watch for.** The approval no longer holds once a file changes:
the change is read and approved again (`approve-again`), which `diff`
makes cheap. Before 1.0.0 a breaking change steps the minor number. A
change carries its reason; the specification carries no change log inside
a file: a decision says why an element is as it is, and the records and
git say what changed when.

**Deeper.** `docs/maintenance.md` (Change requests, Releases and versions,
The diff verb), `docs/changes.md` (SpecArch's own register, generated).

<!-- situation: large-change-late -->
## A large change late in the life cycle

**When it happens.** Weeks before go-live, the customer asks for something
that reaches a dozen elements: a second kind of member, a new approval
step in every workflow, a field every screen shows. Everybody wants to
know what it touches before anybody says yes.

**What to run.**

    specarch document changes --out docs spec
    specarch decision rests ADR-001 spec
    specarch gaps spec
    specarch decide Q-9 --by library-board --answer "<the option chosen>" --record COM-7 spec
    specarch diff spec-at-approval spec
    specarch document releases --out docs spec

The change request is written and analysed: `affects` names what it adds,
changes and removes. Under each open change the changes document shows
what it reaches in the specification as it is now: the elements its
pointers name, the tests about them, the pages, operations, commands and
channels that read an entity among them, the code targets that take a
section it touches, the documents that show one, and the approval it
voids. `decision rests` prints the decisions that rest on a decision the
change would overturn. The analysis ends in a decision that names the
change under `answers`, in an open question the change names, or in a
rejection on the record.

**A worked example.** What rests on the library's first decision, and what
an open change reaches:

    $ specarch decision rests ADR-001 examples/library-lending
    2 decisions rest on ADR-001, nearest first
    ADR-001: Late fees are a flat daily rate capped at the replacement cost (accepted, 2026-10-07)
    ADR-003: The late fee is computed once, at return, and stored on the loan (accepted, 2026-10-25; rests on ADR-001, ADR-002; from idea IDEA-1)
    ADR-004: A waiver is a request of its own that names the amount and the reason, and the fee stands until the request is approved (accepted, 2026-10-25; rests on ADR-003)

The changes document of the library lists CR-3 among the open items
with its phase, type, urgency, impact and cause, and under it what the
change reaches, before anything changes.

**What to watch for.** A change with `impact: major` after 1.0.0 steps the
major version, and the release's version step must be at least as large as
its largest change: `diff` checks it. An approval without a written impact
is the usual way a small request breaks an interface: analysed is a status
of its own. The elements a decision changes carry `origin: decided` and
`decidedIn`, and the decision says why.

**Deeper.** `docs/acceptance.md` (4, What it reaches, before it changes),
`docs/maintenance.md`, `docs/decision-tracking-2026-10-25.md`.

<!-- situation: absent-user-objects -->
## A user who was absent objects at the acceptance test

**When it happens.** The desk staff were not at the requirements meetings;
the board and the IT people were. At the user acceptance test the desk
sees the screen for the first time and says the action it uses most is
below the fold.

**What to run.**

    specarch validate spec
    specarch log spec --recorded-by librarian --direction in --from librarian --to library-it --at 2026-10-21T10:40:00Z --channel meeting --type objection --words "..." --form summary --session TS-1 --element "#/pages/loans-list" --change CR-3
    specarch document changes --out docs spec
    specarch decide Q-12 --by library-board --answer "<the option chosen>" --record COM-5 spec
    specarch decision why ADR-005 spec

Before the test, `validate` warns `unheard` at a stakeholder an accepted
requirement or an approved page relies on that no communication, session,
interview or decision shows was ever heard. At the test, the remark is
logged as an objection, then raised as a change request with `phase:
acceptance`, the build it is against (version, environment, digest,
build), `seenAt` copied from the build's trace, `observed` and `expected`
in the tester's words, and the record linked. Its analysis writes the
`cause`: `omitted` (the specification left it out), `wrong` (it said it,
and that was not what was meant), `changed` (the need moved) or `unheard`
(the role that uses it was absent or never asked), and ends in a decision
that names the change, a question, or a rejection. `decision why` prints
the chain a decision rests on.

**A worked example.** A specification kept with records, two roles never
heard:

    $ specarch validate project
    project/specarch.yaml:5:9: warning: /stakeholders/desk: unheard: desk was never heard: no communication record names it, no session lists it present, no interview lists it and no decision is by it, while requirement CASE-1 and page members-list rely on it; hear it before acceptance, and record what it said with specarch log [unheard@specarch.yaml#/stakeholders/desk]
    project/specarch.yaml:7:10: warning: /stakeholders/board: unheard: board was never heard: no communication record names it, no session lists it present, no interview lists it and no decision is by it, while requirement CASE-1 relies on it; hear it before acceptance, and record what it said with specarch log [unheard@specarch.yaml#/stakeholders/board]

In the library example the chain is on the timeline: SES-1, the renewals
meeting, lists `library-it` invited and absent; TS-1 is the acceptance
test of 0.2.0 on staging; COM-4 is the desk's objection there; CR-3 is the
change request against build 0.2.0 at staging, `seenAt:
["#/pages/loans-list"]`, and ADR-005 is the decision that approved it:

    $ specarch decision why ADR-005 examples/library-lending
    ADR-005 rests on no decision
    ADR-005: The actions of the loans list stand in the order the desk uses them, Renew first (accepted, 2026-10-28)

**What to watch for.** Unheard is kept apart from omitted because its
remedy is different: not a better pen, but a chair at the table for the
role the warning names, before the test. A remark is applied to the
specification, never to the generated code, which the next generation
overwrites; the trace is what lets the remark name the element. A defect
is measured against the specification; a change against what the users
need, so a remark the specification already answers is a defect.

**Deeper.** `docs/acceptance.md` (3, A remark is a change request; the
cause table), `docs/maintenance.md` (Change requests), the library
example's `docs/timeline.md` and `docs/changes.md`.

<!-- situation: approve-again -->
## Approving again, with diff

**When it happens.** The specification was approved, then changed: a
change request carried out, a question answered, a format upgraded. The
approver will not read three hundred pages again for a column.

**What to run.**

    specarch diff spec-at-approval spec
    specarch decision why ADR-005 spec
    specarch approve --by librarian --session SES-2 spec

The approval record holds the commit it was made at. `diff` between that
commit and the working tree lists every changed element with the change
request or defect that covers it, its version check skipped when both
sides carry the same `info.version`. The approver reads the list, not the
specification, and approves again, whole.

**A worked example.** One pre-release compared with itself, one enum
value added:

    $ specarch diff old/spec new/spec
    minor changed #/enums/Colour: gains the value blue

**What to watch for.** Approval stays whole: the digest is over every
file, because approval is of exactly what was read; a part of a
specification is not approved. A changed element no change request or
defect covers is an error of `diff`: raise the record first. A release
record with status planned must exist for the version being built.

**Deeper.** `docs/acceptance.md` (8, What the owner decided),
`docs/maintenance.md` (The diff verb), `specarch help diff`,
`specarch help approve`.

<!-- situation: release -->
## Release and release notes

**When it happens.** The changes for 0.2.0 are carried out and tested, and
the people who run the system want to know what is in it, how to install
it, how to roll it back and who signed it off.

**What to run.**

    specarch diff spec-0.1.0 spec
    specarch document releases --out docs spec
    specarch document deployment --out docs spec
    specarch document commissioning --out docs spec
    specarch document handover --out docs spec
    specarch document manual --out docs spec
    specarch document timeline --out docs spec
    specarch document progress --out docs spec

While a release is being built, `info.version` carries a pre-release tag
(`0.2.0-dev`) and a release record with status planned exists for it; on
release the tag is dropped in the same commit that sets the record to
released, naming the change requests and defects it `includes`. `diff`
between the two tagged specifications checks the version step against the
largest change and that every changed element is covered. The release
notes group each release's changes as added, changed, removed and fixed,
with where each was raised and its cause, and count the changes by cause;
the commissioning procedure is the sign-off sheet; the handover document
is for the engineers of a customer who runs an environment from the
source; the user manual is for the people who use the system. A deploy is
a record, `DEP-<n>`, one installation on one environment at one time.
The progress report counts what the specification and the records hold:
the elements by stage, the open questions by priority and deciding role,
the unheard roles, whether the stakeholder list is confirmed, the hours a
change request records in `effort` (`estimatedHours`, `spentHours`) with
the ones that record none named as missing, and a burn-up of the elements
approved and generated per release. It estimates nothing.

**A worked example.** The library's release notes:

    $ specarch document releases --out docs examples/library-lending
    specarch document: 1 file written, 0 files already current
    $ sed -n 7,19p docs/releases.md
    ## 0.2.0, planned

    ### Added

    - CR-1, Let a member renew a loan once: `#/tests/renew-a-loan`; raised in production by member; cause omitted (the specification left it out).

    ### Changed

    - CR-1, Let a member renew a loan once: LIB-4, `#/entities/Loan`; raised in production by member; cause omitted (the specification left it out).

    ### Fixed

    - DEF-1, The late fee is not capped at the replacement cost: violated LIB-5; raised in production by librarian.

**What to watch for.** A release with many changes caused omitted, wrong
or unheard says the requirements work left gaps; one with many changed
says only that the need moved. The commissioning run that accepted the
release and the deploy that installed it are records, so the timeline
shows the order: commissioning passed, then deployed. The public
interface of SemVer is the paths, the commands, the channels, the routes
and the settings; a client written for the previous version must not break
on a minor step.

**Deeper.** `docs/maintenance.md` (Releases and versions), `docs/releases.md`
(SpecArch's own notes, generated), `docs/commissioning.md`,
`docs/deployment.md`.

<!-- situation: formats-move -->
## A release of SpecArch moves the formats

**When it happens.** A new release of the program reads a specification
written for an earlier meta-model and warns at its version line. Six
formats each carry a version on the first line of their files: the
meta-model (`specarch`), the implementation file
(`specarchImplementation`), the record (`specarchRecord`), the idiom
(`specarchIdiom`), the estate (`specarchEstate`) and the vocabulary
(`specarchVocabulary`).

**What to run.**

    specarch version
    specarch upgrade --dry-run spec
    specarch upgrade spec
    specarch validate spec
    specarch skill --out .agents/skills

`upgrade` rewrites only the version lines, the editor hints and the keys
the new version renamed, in place, keeping comments, order and quoting;
what it cannot do mechanically it lists as a step left to the author and
exits 1. A second run changes nothing.

**A worked example.**

    $ specarch version
    specarch 1.5.0
    specifications: meta-model 1.0.0 and any later 1.0 patch, and 0.3, 0.2 and 0.1 with a warning
    implementation files, records, idioms and estates: 1.0.0 and any later 1.0 patch, and 0.1 with a warning
    vocabularies: 1.0.0 and any later 1.0 patch, and 0.1
    $ specarch upgrade --dry-run project
    project/deployment/handover.yaml:7:3: "source" -> "sourceDelivery"
    project/design/roles.yaml:20:5: rows -> rowPolicies
    project/specarch.yaml:3:11: '0.3' -> '1.0.0'
    ... 7 more changes to version lines and editor hints
    specarch upgrade: 10 changes to make in 6 files, 0 steps left to the author

**What to watch for.** Writing a file changes the specification, so the
approval no longer holds: approve again after reading the documents
(`approve-again`). Pin the program: record the module version and
checksum `go version -m` prints in the repository's agent instructions.
Write the agent skill again after an upgrade, so no reference names a
command the release no longer has.

**Deeper.** `docs/conventions.md` (Meta-model versions),
`specarch help upgrade`, `specarch help version`, the release notes of
each release of SpecArch.

<!-- situation: agents -->
## Agents driving SpecArch

**When it happens.** An agent in a chat, or in a CI job, does the work: it
writes the specification from sources, asks the questions, records the
answers and generates the code, and the owner decides.

**What to run.**

    specarch guide --agent
    specarch capabilities --json
    specarch help situations
    specarch model spec
    specarch validate --json spec
    specarch mcp
    specarch skill --out .agents/skills

`guide --agent` prints the order of work, how to read a problem, when to
stop and ask the owner, the limits and the conventions; `AGENTS.md` in
this repository holds the same text. `capabilities --json` lists every
command as a tool with a JSON Schema for its inputs, its outputs and its
exit codes, and the plug-ins on PATH. `--json` on any command prints one
object; `--dry-run` on any command that writes lists what it would write.
`model` prints the specifications as one JSON document for a program.
`mcp` serves every command as a tool of the Model Context Protocol over
standard input and output; `skill` writes an agent skill with the guide
and one reference per command. `specarch help agents` explains both ways
in.

**A worked example.**

    $ specarch skill --out skills --check
    specarch skill --check: 28 files checked, 0 files differ
    $ specarch model examples/library-lending | head -c 120
    {"estates":[],"implementations":[],"program":"1.5.0","specarchModel":"1.0.0","specifications":[{"document":{"accessibility":

**What to watch for.** The program asks nothing and prompts for nothing: a
step that needs a person is a command the person runs (`decide`,
`approve`). An agent stops and asks at every open must or should question,
at any choice between two sources that disagree, before approving, before
generating a draft for anything that will be used, and before overwriting
a file built by hand. It records who said what with `specarch log` and
guesses neither who nor when. An MCP server that lists its tools without a
session, the stateless revision of the protocol, is coming; a client that
speaks both falls back to the handshake now.

**Deeper.** `AGENTS.md`, `docs/from-sources.md` (5, The agent's
instructions), `specarch help agents`, `specarch help conventions`,
`specarch help mcp`, `specarch help skill`.
