# From an old document to code: partial specifications, open questions and approval

A specification is rarely written from nothing. Most start from what exists:
a prose document, a running system, a folder of code. SpecArch reads such a
source into a specification that says exactly what the source supports, marks
how each element is known, lists what is still unknown as questions to the
people who can answer them, and refuses to generate code until the questions
are answered and the people the system is for have read and approved the
result in document form. This document is the design of that path. Most of it is built; the
last section says what is built and what the dispatcher queues next.

The path is:

    old document  ->  specification  ->  validation  ->  refinement  ->  refined documents  ->  code
                      extract, or an      validate        decide, and       document             approve,
                      agent by hand       gaps            edit the spec     (and gaps)           generate

- Old document: the prose documents of an existing system, or its code, are
  the input. They are declared as sources of the specification, so that
  every element can cite where it came from.
- Specification: the partial specification extracted from them. Every
  element carries its origin (stated, inferred or decided). What the source
  does not say is not invented: it is written as an open question.
- Validation: `specarch validate` checks the specification as always, and
  counts the open questions. `specarch gaps` lists them by stage, with what
  each blocks, who decides it, and which outputs are possible now.
- Refinement: the deciding stakeholder answers a question with a decision or
  with material. The answer becomes a decision record, the affected elements
  are written or corrected, the question is removed, and the specification is
  validated again. The list of questions shrinks until nothing blocks.
- Refined documents: `specarch document` writes the human documents from the
  specification, open questions marked in place. The stakeholders read
  them. That is how a specification is reviewed: in the form its readers
  will use.
- Code: `specarch approve` records that a stakeholder read current
  documents and approves this exact specification; `specarch generate` refuses to run
  without that record, or while a question blocks what the target reads.

The standards behind it are ISO/IEC/IEEE 29148:2018, whose rule for a complete
set of requirements is that it holds no "to be defined", "to be specified" or
"to be resolved" clause, and that resolving them is iterative within a time
set by risk (5.2.6); the same standard counts closure of such items as a
measure of the requirements work (6.6.3) and makes requirements validation
subject to approval by the project authority and the key stakeholders
(6.3.3.6). IEEE Std 830-1998, which 29148 replaced, said what a "to be
determined" item must carry: why it is open, what must be done to close it,
who is responsible and by when (4.3.3). An open question in SpecArch is that
item, with those fields. The decision that closes one is an architecture
decision record in the form of Michael Nygard.

## Why a question, and not a placeholder

The Low IQ Tax principle (`docs/principles.md`, rule 6) forbids placeholders:
nothing half-defined is accepted as defined. A value typed in to make the
schema pass ("TBD", "14 days (check this)", a made-up permission) is exactly
the thing the rule is against, because it looks like data and every tool
downstream treats it as data.

An open question is the opposite of a placeholder. It says, in the
specification itself, that something is not known, what it is, who decides
it, and what cannot be finished until it is. The validator treats the
question as the only licence for an element to be incomplete: a required key
may be missing exactly where a question says it is unknown, and nowhere
else. So the "partial mode" the requirement asked for needs no switch. A specification with
open questions is a partial specification; one without is complete. There is
nothing to turn on, and no second set of rules to learn (principle 8).

## Open questions

`questions` is a section of the specification. Each question is a named
object whose key is an ID in the usual form, such as `Q-12` or `OPEN-3`.

| Field | Holds |
|---|---|
| `question` | what is asked, as one plain question |
| `kind` | `decision`: the stakeholder decides; `material`: the stakeholder provides something to read (a document, a file, a screenshot, a record) |
| `priority` | `must`: what it blocks is not defined, and nothing that reads it can be generated; `should`: what it blocks is written as inferred and the stakeholder should confirm it before it is built on; `could`: the answer would improve the specification, but nothing waits for it |
| `blocks` | what cannot be final until it is answered: a stage name, a section name, or a pointer to an element (`#/entities/Loan`, `#/paths/~1loans/post`) or to a key of one (`#/requirements/BR-3/priority`) |
| `decidedBy` | the stakeholder who decides or provides, by key; a role, never a person |
| `options` | for a decision with a known set of answers: the answers, two or more, so that the question can be put as a choice |
| `names` | for a question that blocks one key a reader left out: the name the source gives there, such as the operation a workflow's trigger names, which only another surface declares; `specarch merge` writes it at the key and leaves the question out once a tree declares an element of that name |
| `why`, `cites` | why the question arises, and where in the source the ambiguity is |

The three priorities follow 29148's note that the time to resolve an open
item is set by risk and dependency. `must` is the item everything depends
on, `could` the one nothing does.

### Where a question is written

A question is written in the folder of the stage it is about, with the
elements it blocks: a question about a requirement under `requirements/`,
one about an entity under `design/`. A stage that has no folder has its
questions in the root file, like its other sections. `questions` is the one
section that every stage folder may hold; the loader merges them into one
section, and the validator checks that a question sits in the stage of what
it blocks (`question_stage`). A question that blocks two stages is two
questions.

### What `blocks` may name

- A stage or section name, such as `tests` or `permissions`: the stage or
  section as a whole is not yet written, or not yet complete. Such a block
  covers no missing key; it says which outputs wait.
- A pointer to an element, `#/<section>/<name>`: the element exists by name
  but is not, or not fully, described. The element is written as far as it
  is known, down to an empty mapping when nothing but its name is known.
  The validator covers every required key missing at or under that pointer,
  and the warnings about the element.
- A pointer to a key of an element, `#/requirements/BR-3/priority`: that one
  key is unknown. The validator covers the missing key, and nothing else of
  the element.

An element that is not even known by name is not blocked by pointer; the
question blocks its section and names it in the question text. Writing an
element as an empty mapping and blocking it, or blocking its section, are
not two ways to say one thing: the first says the element exists and is
referred to (a relation may target it), the second says the section is not
yet complete.

The pointer's section and name must resolve; after them, the key tokens must
resolve down to at most one missing leaf, which is the key the question is
about (`question_block`). The covering applies to `must` questions only: a
`should` question is about an element that is written, so a key missing at
its pointer is an ordinary error.

### What the validator does with them

- `validate` prints no line per open question. Authors and agents run it
  after every edit, and a list of a hundred known questions would bury
  the errors. The count goes into the summary line on standard error:
  `specarch: 1 input checked: 0 errors, 2 warnings, 12 open questions`.
- A required key missing at a pointer a `must` question blocks is covered: it
  is not reported, and `gaps` lists it under the question instead. So is a
  warning about the blocked element (its acceptance criteria, its
  traceability, its missing test cases). A wrong value next to the gap is
  still an error.
- The rules: `question_block` (every `blocks` entry is a stage, a section,
  or a pointer that resolves as described), `stakeholder` (`decidedBy` is a
  stakeholder of the specification), `question_stage` (the question sits in
  the stage of what it blocks) and `question_answered` (an accepted decision
  that `answers` a question which is still open; below).

## Origin: how an element is known

Every element that may carry `why` and `cites` may carry `origin`:

| `origin` | Means | Must also carry |
|---|---|---|
| `stated` | the source says it; the element paraphrases the source | `cites`, at least one citation, naming the source and where in it (`origin_citation`) |
| `inferred` | the source does not say it; it was concluded from evidence, such as code, data or the source's silence | `why`, saying from what and how (`origin_reason`) |
| `decided` | a stakeholder decided it, answering a question or correcting the source | `decidedIn`, the decision record (`origin_decision`) |

The three words the requirement used were stated, inferred and missing. Missing is not a
value of `origin`, because a missing element has no element to put the value
on: the open question is the missing mark, and it carries what a mark could
not (who decides, what it blocks). The third value is instead `decided`,
which is where every answered question ends up, so that a reader can tell
what the source said from what a stakeholder settled.

An element without `origin` was written spec-first, as every element of
SpecArch's own specification is. A specification built from sources says
so once, in its root file, with `info.tracksOrigin: true`; from then on every
element of a section that has no `origin` is reported, as a warning
(`origin_missing`), since the stage's unfinished work is to say where each
thing came from. The trigger is declared, not inferred from the presence of
one `origin` somewhere, so that one marked element never turns a quiet
specification into three hundred warnings. The warning covers the named
objects of the sections; a field, a parameter or a step inherits the origin
of the element that holds it unless it says otherwise.

In the documents, an element's origin is one line before its Insight and
Notes: `Origin: stated in <source>, clause <x>`, `Origin: inferred`
(the Insight then says from what), or `Origin: decided in ADR-021`.

## The decision that closes a question

An answer is recorded as a decision in `decisions`, the architecture
decision records the specification already keeps, with two more fields:

| Field | Holds |
|---|---|
| `answers` | the questions this decision answers, by ID |
| `decidedBy` | the stakeholder who decided, by key; required with `answers` |

The record's `context` carries the question, `decision` the answer,
`consequences` the elements written or changed because of it, `date` when,
and `cites` the material the answer rests on. An answer the stakeholder gave
in words, typed in the agent queue's panel or said in a meeting, is declared
as a source of kind `interview` with its date; a document they provided is a
source of kind `document`. The affected elements then carry
`origin: decided` and `decidedIn: ADR-021`, and the question is removed from
the specification, because it is no longer true that the thing is unknown.

So who, when and from what are the decision's `decidedBy`, `date` and
`cites`; the question's ID lives on in `answers`, as text, since the question
itself is gone. The validator refuses an accepted decision that `answers` a
question still present (`question_answered`): either the question is open,
and the decision is `proposed`, or it is answered, and the question goes.

Decisions follow Nygard's form because they already did, and because the
form fits: an answered question is a decision with a context (the question),
a decision (the answer) and consequences (what changed). A decision is
specification, not a record under `records/`, for the same reason the
existing decisions are: it describes why the system is as it is now, and a
later decision supersedes it rather than editing it. A change to an approved
specification is a change request, as `docs/maintenance.md` designs; an
answer to an open question of an unapproved one is not, since nothing was
promised yet.

## Approval

A specification is approved by reading its documents. The approval is a
record, since it happens once per version:

    records/approvals/<version>.yaml

    specarchRecord: "0.1"
    kind: approval
    version: 1.0.0
    approvedBy: product-owner
    date: "2026-10-08"
    documents: [techspec, requirements, testplan]
    digest: sha256:9f86d081...

`specarch approve --by <stakeholder> [--date <date>] <folder>` writes it,
after checking that the specification has no error, that no `must` or
`should` question is open, that `--by` is a stakeholder of the specification,
and that at least one document target is configured and every configured
document on disk is what the specification generates now, so that what the
approver read is what is approved. The date is today unless given. The record
lives beside the specification's folder, under `records/`, where the
commissioning records and the records of `docs/maintenance.md` live.

The digest is over the specification's files, not its version: the version
does not change while a specification is being written, and an approval must
be void the moment a file changes. It is the SHA-256 of every `.yaml` file
under the specification's folder, taken in byte order of their paths
relative to that folder, each as its path, a zero byte, its bytes and a zero
byte. Any edit to any of those files, a comment included, voids the
approval, and the approver reads and approves again.

In 29148's words, the approved specification is a baseline (clause 3, a
formally approved version of a configuration item), and requirements
validation is subject to approval by the project authority and the key
stakeholders (6.3.3.6). The record names the role that gave it.

The record's shape is checked by `approve`, which writes it, and by
`generate`, which reads it. The record schema that `docs/maintenance.md`
plans will hold it beside the other kinds; until then the shape is fixed here
and in `docs/conventions.md`.

## The gate on code generation

`specarch generate <target>` validates first, as it always did, then:

1. If a `must` or `should` question blocks a section or element the target
   reads, it refuses with status 1 and names the questions. What a target
   reads is declared in the implementation file, under `targets.<name>.reads`,
   as a list of sections; a target that does not declare it reads every
   section. The plug-in protocol has no word for what a plug-in reads, so the
   declaration sits with the target's other settings, where the person who
   installed the plug-in writes it.
2. If `records/approvals/<version>.yaml` is missing, or its digest is not the
   digest of the files now, it refuses with status 1 and says why, and how to
   approve. `--unapproved` lets it through, on purpose and visibly; the
   refusal is the default.

`could` questions hold nothing. The documents are never refused: a
specification with open questions is exactly what the documents must show,
so that the people who decide can see them.

## The readiness report

`specarch gaps <folder>...` prints the open questions and what they hold up,
summary first:

- the counts: open questions by priority, and when the specification tracks
  origin, the elements by origin and the elements without one;
- the questions, grouped by stage in life-cycle order and within a stage by
  priority then ID, each with its kind, who decides, what it blocks (and for
  a blocked element, which keys are missing), its options, its Insight and
  Notes;
- the outputs: one row per document target and per code target the
  implementation files name, saying whether it is ready, a draft (the
  questions that concern it), or waits (the questions, and the approval that
  is missing or void).

It exits 0 when no `must` or `should` question is open, 1 when one is, and 2
on a usage error or a specification with errors. The same text is the
document target `questions`, written as `questions.md` into the documents'
folder by `specarch document questions`, so that the open questions are
handed over with the other documents and read by the stakeholders who decide
them. A specification without open questions gets a one-paragraph document
saying so; that is a fact worth handing over, not a placeholder.

## The documents during refinement

Every document shows the open questions where they bite:

- Under an element's heading, or after a table labelled with the row, after
  the Origin line and before the Insight: `Open question Q-12 (must,
  decision): What is the loan period for a reference copy? Decided by the
  head librarian.` One paragraph per question that blocks the element or a
  key of it.
- At the top of a document whose sections a question blocks, after the
  summary paragraph: `Draft: 3 open questions concern this document (Q-3,
  Q-7, Q-12); see the open questions document, or run specarch gaps.`

Which sections a document reads is fixed in the documentor: the requirements
specification reads the requirements stage; the test plan the tests and the
requirements; the traceability matrix the needs, requirements, tests, checks,
monitors and every design section; the deployment guide the deployment
stage and the monitors; the commissioning procedure the checks and the
sign-off; the technical specification everything. A question that blocks a
stage concerns every document that reads a section of it.

## Fit with the agent queue

The agent that builds a specification from a source writes the questions as
it goes, and ends its item with `specarch gaps`. The dispatcher turns each
`must` and `should` question into one question to the person it is for,
with the options when the question has them, the `decidedBy` role saying
whose answer it is, and `blocks` saying what waits. The answer comes back
to the agent, which records the decision, edits the elements, removes the question
and validates; nothing is re-extracted. A `could` question is listed in the
report and asked when there is time for it.

For that the report needs a machine-readable form. `specarch gaps --json`
(one object per question, with its fields, its stage and the missing keys of
what it blocks, and the outputs table) is a follow-up item, as is the
dispatcher's side of it. Until then the dispatcher reads the text.

## Old document against refined document

Before approving, the reviewer wants to see what the refinement changed
against the old document: what was clarified, added, corrected or dropped, and why.
The documentor can derive that from what this design adds, without a
separate comparison format:

| Bucket | How it is known |
|---|---|
| kept | elements with `origin: stated`, with the clause of the old document they cite |
| clarified | elements with `origin: inferred`: the old document left them implicit; the Insight says from what they were concluded |
| added or corrected | elements with `origin: decided`: added when the decision's context says the source was silent, corrected when it says the source was wrong; the decision's `why` is the reason |
| dropped | clauses of the old document that no element cites |

The fourth bucket needs the outline of the old document, so that an
uncited clause can be found. A source lists it as its `clauses`, each a
number and a title, and `specarch gaps` already shows, per source, the
elements each clause produced and the clauses that produced nothing
(`docs/from-sources.md`). The document target `comparison` that writes the
four buckets side by side is a follow-up item; the first three buckets need
nothing beyond `origin`, `cites` and the decisions.

## Stack-neutral and stack-specific

| In the specification (stack-neutral) | In the implementation file (one stack) |
|---|---|
| questions, their priorities and what they block | which sections each code target reads (`targets.<name>.reads`) |
| origin and the decisions that answer questions | |
| the approval record, beside the specification | |

## What this changes in the stage design, and why

- A section `questions` that every stage folder may hold. The one exception
  to "one section, one stage", because a question belongs with what it is
  about, and questions arise at every stage.
- `origin` and `decidedIn` on every element that carries `why` and `cites`,
  in the specification and in implementation files; `tracksOrigin` in
  `info`.
- `answers` and `decidedBy` on a decision.
- `reads` on a target of an implementation file.
- The record kind `approval`, under `records/approvals/`.
- Verbs: `gaps` and `approve`; `generate` gains `--unapproved` and the gate;
  `document` gains the target `questions`.
- Rules: `question_block`, `question_stage`, `question_answered`,
  `origin_citation`, `origin_reason`, `origin_decision` and the warning
  `origin_missing`. `decidedBy` uses the existing `stakeholder` rule.

## The Go and the Swift build

Everything the validator does is in both builds, with the same output: the
loader's handling of `questions`, the seven rules, the covering of missing
keys and warnings, and the count of open questions. `gaps`, `approve` and
the `questions` document are verbs in the family of `document` and
`generate`, which the Swift build does not have; it answers them with status
2, as it does those, and its implementation file says so.

## Implementation items

Built in the item that wrote this design, specification first:

1. The schema: `questions`, `origin`, `decidedIn`, `tracksOrigin`,
   `answers`, `decidedBy` and `reads`; both validator builds read
   `questions` from any stage folder; the seven rules, the covering, the
   count; conformance cases for each.
2. `specarch gaps` and the document target `questions`; the Origin line, the
   open-question paragraphs and the draft notice in every document.
3. `specarch approve`, the approval record and the gate in `generate`, with
   `--unapproved` and `reads`.
4. `docs/stages.md`, `docs/conventions.md`, `docs/generators.md`,
   `docs/extraction.md`, the README and the roadmap.

For the dispatcher to queue, in this order:

1. `specarch decide <question> --by <stakeholder> --answer <text>
   [--source <key>]`: writes the decision record with the next ADR number,
   removes the question from its file, and prints the elements it blocked so
   the agent edits them. Until then the agent writes the decision by hand and
   the validator holds it to the rules.
2. `specarch gaps --json`, and the dispatcher turning its questions into
   questions to the people who decide, with options, and feeding the
   answers back.
3. The document target `comparison` with the four buckets above, and a
   validator rule for a citation that names a clause its source does not
   list.
4. The approval record in the record schema of `docs/maintenance.md`, when
   that schema is built (its item 2), with the rule that an approval's
   version is the specification's.
5. `reads` declared by a plug-in itself, in its answer to a probe request,
   once a plug-in exists that would use it.
